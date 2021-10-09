package api

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
)

type SealerClient struct {
	AllocateSector func(context.Context, AllocateSectorSpec) (*AllocatedSector, error)

	AcquireDeals func(context.Context, abi.SectorID, AcquireDealsSpec) (Deals, error)

	AssignTicket func(context.Context, abi.SectorID) (Ticket, error)

	SubmitPreCommit func(context.Context, AllocatedSector, PreCommitOnChainInfo, bool) (SubmitPreCommitResp, error)

	PollPreCommitState func(context.Context, abi.SectorID) (PollPreCommitStateResp, error)

	SubmitPersisted func(context.Context, abi.SectorID, string) (bool, error)

	WaitSeed func(context.Context, abi.SectorID) (WaitSeedResp, error)

	SubmitProof func(context.Context, abi.SectorID, ProofOnChainInfo, bool) (SubmitProofResp, error)

	PollProofState func(context.Context, abi.SectorID) (PollProofStateResp, error)

	ListSectors func(context.Context) ([]*SectorState, error)

	ReportState func(context.Context, abi.SectorID, ReportStateReq) error

	ReportFinalized func(context.Context, abi.SectorID) error
}
