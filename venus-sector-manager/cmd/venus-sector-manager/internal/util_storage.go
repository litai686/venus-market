package internal

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dtynn/dix"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs-force-community/venus-cluster/venus-sector-manager/api"
	"github.com/ipfs-force-community/venus-cluster/venus-sector-manager/dep"
	"github.com/ipfs-force-community/venus-cluster/venus-sector-manager/modules/util"
	"github.com/ipfs-force-community/venus-cluster/venus-sector-manager/pkg/objstore"
	"github.com/ipfs-force-community/venus-cluster/venus-sector-manager/pkg/objstore/filestore"
	"github.com/urfave/cli/v2"
)

var utilStorageCmd = &cli.Command{
	Name: "storage",
	Subcommands: []*cli.Command{
		utilStorageAttachCmd,
		utilStorageFindCmd,
	},
}

var utilStorageAttachCmd = &cli.Command{
	Name: "attach",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "name",
		},
		&cli.BoolFlag{
			Name: "strict",
		},
		&cli.BoolFlag{
			Name: "read-only",
		},
		&cli.BoolFlag{
			Name: "use-cache-dir",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
		},
	},
	ArgsUsage: "<storage path>",
	Action: func(cctx *cli.Context) error {
		gctx, gcancel := NewSigContext(cctx.Context)
		defer gcancel()

		storPath := cctx.Args().First()
		if storPath == "" {
			return fmt.Errorf("storage path is required")
		}

		abs, err := filepath.Abs(storPath)
		if err != nil {
			return fmt.Errorf("get abs path: %w", err)
		}

		verbose := cctx.Bool("verbose")
		name := cctx.String("name")
		strict := cctx.Bool("strict")
		readOnly := cctx.Bool("read-only")

		scfg := filestore.Config{
			Name:     name,
			Path:     abs,
			Strict:   strict,
			ReadOnly: readOnly,
		}

		store, err := filestore.Open(scfg)
		if err != nil {
			return fmt.Errorf("open file store: %w", err)
		}

		name = store.Instance(gctx)
		logger := Log.With("name", name, "strict", strict, "read-only", readOnly)

		cfgExample := struct {
			Common struct {
				PersistStores []filestore.Config
			}
		}{}

		cfgExample.Common.PersistStores = append(cfgExample.Common.PersistStores, scfg)

		var buf bytes.Buffer
		enc := toml.NewEncoder(&buf)
		enc.Indent = ""
		err = enc.Encode(&cfgExample)
		if err != nil {
			return fmt.Errorf("encode example config for storage path: %w", err)
		}

		targetPath := util.SectorPath(util.SectorPathTypeSealed, abi.SectorID{})
		if cctx.Bool("use-cache-dir") {
			targetPath = util.SectorPath(util.SectorPathTypeCache, abi.SectorID{})
		}

		matchPattern := filepath.Join(abs, filepath.Dir(targetPath), "*")
		if verbose {
			logger.Infof("use match pattern %q", matchPattern)
		}

		matches, err := filepath.Glob(matchPattern)
		if err != nil {
			return fmt.Errorf("find matched files with glob pattern %q", matchPattern)
		}

		sids := make([]abi.SectorID, 0, len(matches))
		for _, mat := range matches {
			base := filepath.Base(mat)
			sid, ok := util.ScanSectorID(base)
			if ok {
				sids = append(sids, sid)
			}

			if verbose {
				logger.Infof("path %q matched=%v", base, ok)
			}
		}

		var indexer api.SectorIndexer

		stopper, err := dix.New(
			gctx,
			DepsFromCLICtx(cctx),
			dep.Product(),
			dix.Override(new(dep.GlobalContext), gctx),
			dix.Populate(dep.InvokePopulate, &indexer),
		)

		if err != nil {
			return fmt.Errorf("construct sector indexer: %w", err)
		}

		defer stopper(gctx) // nolint:errcheck

		for _, sid := range sids {
			err := indexer.Update(gctx, sid, name)
			if err != nil {
				return fmt.Errorf("update sector index for %s: %w", util.FormatSectorID(sid), err)
			}

			if verbose {
				logger.Infof("sector indexer updated for %s", util.FormatSectorID(sid))
			}
		}

		logger.Infof("%d sectors out of %d files have been indexed", len(sids), len(matches))
		logger.Warn("add the section below into the config file:")
		fmt.Println("")
		fmt.Println(strings.TrimSpace(strings.ReplaceAll(buf.String(), "[Common]", "")))

		return nil
	},
}

var utilStorageFindCmd = &cli.Command{
	Name:      "find",
	ArgsUsage: "<actor id> <number>",
	Action: func(cctx *cli.Context) error {
		gctx, gcancel := NewSigContext(cctx.Context)
		defer gcancel()

		args := cctx.Args()

		if args.Len() < 2 {
			return fmt.Errorf("at least 2 args are required")
		}

		mid, err := strconv.ParseUint(args.Get(0), 10, 64)
		if err != nil {
			return fmt.Errorf("parse miner actor id: %w", err)
		}

		num, err := strconv.ParseUint(args.Get(1), 10, 64)
		if err != nil {
			return fmt.Errorf("parse sector number: %w", err)
		}

		var indexer api.SectorIndexer

		stopper, err := dix.New(
			gctx,
			DepsFromCLICtx(cctx),
			dep.Product(),
			dix.Override(new(dep.GlobalContext), gctx),
			dix.Populate(dep.InvokePopulate, &indexer),
		)
		if err != nil {
			return fmt.Errorf("construct deps: %w", err)
		}

		defer stopper(gctx) // nolint:errcheck

		sid := abi.SectorID{
			Miner:  abi.ActorID(mid),
			Number: abi.SectorNumber(num),
		}

		instanceName, found, err := indexer.Find(gctx, sid)
		if err != nil {
			return fmt.Errorf("find store instance for %s: %w", util.FormatSectorID(sid), err)
		}

		if !found {
			Log.Warnf("%s not found", util.FormatSectorID(sid))
			return nil
		}

		Log.Infof("found %s in %q", util.FormatSectorID(sid), instanceName)

		_, err = indexer.StoreMgr().GetInstance(gctx, instanceName)
		if err == nil {
			Log.Infof("store instance exists")
			return nil
		}

		if errors.Is(err, objstore.ErrObjectStoreInstanceNotFound) {
			Log.Warnf("store instance not found, check your config file")
			return nil
		}

		if err != nil {
			return fmt.Errorf("attempt to find store instance: %w", err)
		}

		return nil
	},
}