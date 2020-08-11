package worker

import (
	"sync"

	"github.com/anyswap/CrossChain-Bridge/mongodb"
	"github.com/anyswap/CrossChain-Bridge/tokens"
	"github.com/anyswap/CrossChain-Bridge/tokens/btc"
)

var (
	swapinVerifyStarter  sync.Once
	swapoutVerifyStarter sync.Once
)

// StartVerifyJob verify job
func StartVerifyJob() {
	go startSwapinVerifyJob()
	go startSwapoutVerifyJob()
}

func startSwapinVerifyJob() {
	swapinVerifyStarter.Do(func() {
		logWorker("verify", "start swapin verify job")
		for {
			res, err := findSwapinsToVerify()
			if err != nil {
				logWorkerError("verify", "find swapins error", err)
			}
			if len(res) > 0 {
				logWorker("verify", "find swapins to verify", "count", len(res))
			}
			for _, swap := range res {
				err = processSwapinVerify(swap)
				switch err {
				case nil, tokens.ErrTxNotStable, tokens.ErrTxNotFound:
				default:
					logWorkerError("verify", "process swapin verify error", err, "txid", swap.TxID)
				}
			}
			restInJob(restIntervalInVerifyJob)
		}
	})
}

func startSwapoutVerifyJob() {
	swapoutVerifyStarter.Do(func() {
		logWorker("verify", "start swapout verify job")
		for {
			res, err := findSwapoutsToVerify()
			if err != nil {
				logWorkerError("verify", "find swapouts error", err)
			}
			if len(res) > 0 {
				logWorker("verify", "find swapouts to verify", "count", len(res))
			}
			for _, swap := range res {
				err = processSwapoutVerify(swap)
				switch err {
				case nil, tokens.ErrTxNotStable, tokens.ErrTxNotFound:
				default:
					logWorkerError("verify", "process swapout verify error", err, "txid", swap.TxID)
				}
			}
			restInJob(restIntervalInVerifyJob)
		}
	})
}

func findSwapinsToVerify() ([]*mongodb.MgoSwap, error) {
	status := mongodb.TxNotStable
	septime := getSepTimeInFind(maxVerifyLifetime)
	return mongodb.FindSwapinsWithStatus(status, septime)
}

func findSwapoutsToVerify() ([]*mongodb.MgoSwap, error) {
	status := mongodb.TxNotStable
	septime := getSepTimeInFind(maxVerifyLifetime)
	return mongodb.FindSwapoutsWithStatus(status, septime)
}

func isInBlacklist(swapInfo *tokens.TxSwapInfo) (isBlacked bool, err error) {
	isBlacked, err = mongodb.QueryBlacklist(swapInfo.From)
	if err != nil {
		return isBlacked, err
	}
	if !isBlacked && swapInfo.Bind != swapInfo.From {
		isBlacked, err = mongodb.QueryBlacklist(swapInfo.Bind)
		if err != nil {
			return isBlacked, err
		}
	}
	return isBlacked, nil
}

func processSwapinVerify(swap *mongodb.MgoSwap) (err error) {
	txid := swap.TxID
	var swapInfo *tokens.TxSwapInfo
	switch tokens.SwapTxType(swap.TxType) {
	case tokens.SwapinTx:
		swapInfo, err = tokens.SrcBridge.VerifyTransaction(txid, false)
	case tokens.P2shSwapinTx:
		if btc.BridgeInstance == nil {
			return tokens.ErrNoBtcBridge
		}
		swapInfo, err = btc.BridgeInstance.VerifyP2shTransaction(txid, swap.Bind, false)
	default:
		return tokens.ErrWrongSwapinTxType
	}
	if swapInfo.Height != 0 &&
		swapInfo.Height < tokens.GetTokenConfig(true).InitialHeight {
		err = tokens.ErrTxBeforeInitialHeight
		return mongodb.UpdateSwapinStatus(txid, mongodb.TxVerifyFailed, now(), err.Error())
	}
	isBlacked, errf := isInBlacklist(swapInfo)
	if errf != nil {
		return errf
	}
	if isBlacked {
		err = tokens.ErrAddressIsInBlacklist
		return mongodb.UpdateSwapinStatus(txid, mongodb.SwapInBlacklist, now(), err.Error())
	}
	return updateSwapinStatus(txid, swapInfo, err)
}

func updateSwapinStatus(txid string, swapInfo *tokens.TxSwapInfo, err error) error {
	resultStatus := mongodb.MatchTxEmpty

	switch err {
	case tokens.ErrTxNotStable, tokens.ErrTxNotFound:
		return err
	case tokens.ErrTxWithWrongMemo:
		resultStatus = mongodb.TxWithWrongMemo
		err = mongodb.UpdateSwapinStatus(txid, mongodb.TxCanRecall, now(), err.Error())
	case nil:
		status := mongodb.TxNotSwapped
		if swapInfo.Value.Cmp(tokens.GetBigValueThreshold(true)) > 0 {
			status = mongodb.TxWithBigValue
			resultStatus = mongodb.TxWithBigValue
		}
		err = mongodb.UpdateSwapinStatus(txid, status, now(), "")
	case tokens.ErrTxSenderNotRegistered:
		return mongodb.UpdateSwapinStatus(txid, mongodb.TxSenderNotRegistered, now(), err.Error())
	default:
		return mongodb.UpdateSwapinStatus(txid, mongodb.TxVerifyFailed, now(), err.Error())
	}

	if err != nil {
		logWorkerError("verify", "processSwapinVerify", err, "txid", txid)
		return err
	}
	return addInitialSwapinResult(swapInfo, resultStatus)
}

func processSwapoutVerify(swap *mongodb.MgoSwap) error {
	txid := swap.TxID
	swapInfo, err := tokens.DstBridge.VerifyTransaction(txid, false)
	if swapInfo.Height != 0 &&
		swapInfo.Height < tokens.GetTokenConfig(false).InitialHeight {
		err = tokens.ErrTxBeforeInitialHeight
		return mongodb.UpdateSwapoutStatus(txid, mongodb.TxVerifyFailed, now(), err.Error())
	}
	isBlacked, errf := isInBlacklist(swapInfo)
	if errf != nil {
		return errf
	}
	if isBlacked {
		err = tokens.ErrAddressIsInBlacklist
		return mongodb.UpdateSwapoutStatus(txid, mongodb.SwapInBlacklist, now(), err.Error())
	}
	return updateSwapoutStatus(txid, swapInfo, err)
}

func updateSwapoutStatus(txid string, swapInfo *tokens.TxSwapInfo, err error) error {
	resultStatus := mongodb.MatchTxEmpty

	switch err {
	case tokens.ErrTxNotStable, tokens.ErrTxNotFound:
		return err
	case tokens.ErrTxWithWrongMemo:
		resultStatus = mongodb.TxWithWrongMemo
		err = mongodb.UpdateSwapoutStatus(txid, mongodb.TxCanRecall, now(), err.Error())
	case nil:
		status := mongodb.TxNotSwapped
		if swapInfo.Value.Cmp(tokens.GetBigValueThreshold(false)) > 0 {
			status = mongodb.TxWithBigValue
			resultStatus = mongodb.TxWithBigValue
		}
		err = mongodb.UpdateSwapoutStatus(txid, status, now(), "")
	default:
		return mongodb.UpdateSwapoutStatus(txid, mongodb.TxVerifyFailed, now(), err.Error())
	}

	if err != nil {
		logWorkerError("verify", "processSwapoutVerify", err, "txid", txid)
		return err
	}
	return addInitialSwapoutResult(swapInfo, resultStatus)
}
