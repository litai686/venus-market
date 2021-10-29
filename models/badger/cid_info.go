package badger

import (
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/venus-market/models/repo"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("badgerpieces")

func NewBadgerCidInfoRepo(cidInfoDs repo.CIDInfoDS) repo.ICidInfoRepo {
	return &baderCidInfoRepo{cidInfos: statestore.New(cidInfoDs)}
}

type baderCidInfoRepo struct {
	cidInfos *statestore.StateStore
}

var _ repo.ICidInfoRepo = (*baderCidInfoRepo)(nil)

// Store the map of blockLocations in the PieceStore's CIDInfo store, with key `pieceCID`
func (ps *baderCidInfoRepo) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]piecestore.BlockLocation) error {
	for c, blockLocation := range blockLocations {
		err := ps.mutateCIDInfo(c, func(ci *piecestore.CIDInfo) error {
			for _, pbl := range ci.PieceBlockLocations {
				if pbl.PieceCID.Equals(pieceCID) && pbl.BlockLocation == blockLocation {
					return nil
				}
			}
			ci.PieceBlockLocations = append(ci.PieceBlockLocations, piecestore.PieceBlockLocation{BlockLocation: blockLocation, PieceCID: pieceCID})
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ps *baderCidInfoRepo) ListCidInfoKeys() ([]cid.Cid, error) {
	var cis []piecestore.CIDInfo
	if err := ps.cidInfos.List(&cis); err != nil {
		return nil, err
	}

	out := make([]cid.Cid, 0, len(cis))
	for _, ci := range cis {
		out = append(out, ci.CID)
	}

	return out, nil
}

// Retrieve the CIDInfo associated with `pieceCID` from the CID info store.
func (ps *baderCidInfoRepo) GetCIDInfo(payloadCID cid.Cid) (piecestore.CIDInfo, error) {
	var out piecestore.CIDInfo
	if err := ps.cidInfos.Get(payloadCID).Get(&out); err != nil {
		return piecestore.CIDInfo{}, err
	}
	return out, nil
}

func (ps *baderCidInfoRepo) ensureCIDInfo(c cid.Cid) error {
	has, err := ps.cidInfos.Has(c)

	if err != nil {
		return err
	}

	if has {
		return nil
	}

	cidInfo := piecestore.CIDInfo{CID: c}
	return ps.cidInfos.Begin(c, &cidInfo)
}

func (ps *baderCidInfoRepo) mutateCIDInfo(c cid.Cid, mutator interface{}) error {
	err := ps.ensureCIDInfo(c)
	if err != nil {
		return err
	}

	return ps.cidInfos.Get(c).Mutate(mutator)
}