package worker

import (
	"container/ring"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Bridge/common"
	"github.com/anyswap/CrossChain-Bridge/dcrm"
	"github.com/anyswap/CrossChain-Bridge/params"
	"github.com/anyswap/CrossChain-Bridge/tokens"
	"github.com/anyswap/CrossChain-Bridge/tokens/btc"
)

var (
	acceptSignStarter sync.Once

	acceptRing        *ring.Ring
	acceptRingLock    sync.RWMutex
	acceptRingMaxSize = 500

	retryInterval = 3 * time.Second
	waitInterval  = 20 * time.Second

	// those errors will be ignored in accepting
	errIdentifierMismatch = errors.New("cross chain bridge identifier mismatch")
	errInitiatorMismatch  = errors.New("initiator mismatch")
	errWrongMsgContext    = errors.New("wrong msg context")
)

// StartAcceptSignJob accept job
func StartAcceptSignJob() {
	acceptSignStarter.Do(func() {
		logWorker("accept", "start accept sign job")
		acceptSign()
	})
}

func acceptSign() {
	for {
		signInfo, err := dcrm.GetCurNodeSignInfo()
		if err != nil {
			logWorkerError("accept", "getCurNodeSignInfo failed", err)
			time.Sleep(retryInterval)
			continue
		}
		logWorker("accept", "acceptSign", "count", len(signInfo))
		for _, info := range signInfo {
			keyID := info.Key
			history := getAcceptSignHistory(keyID)
			if history != nil {
				logWorker("accept", "history sign", "keyID", keyID, "result", history.result)
				_, _ = dcrm.DoAcceptSign(keyID, history.result, history.msgHash, history.msgContext)
				continue
			}
			agreeResult := "AGREE"
			err := verifySignInfo(info)
			switch err {
			case errIdentifierMismatch,
				errInitiatorMismatch,
				errWrongMsgContext,
				tokens.ErrNoBtcBridge,
				tokens.ErrTxNotStable,
				tokens.ErrTxNotFound:
				logWorkerTrace("accept", "ignore sign", "keyID", keyID, "err", err)
				continue
			}
			if err != nil {
				logWorkerError("accept", "disagree sign", err, "keyID", keyID)
				agreeResult = "DISAGREE"
			}
			logWorker("accept", "dcrm DoAcceptSign", "keyID", keyID, "result", agreeResult)
			res, err := dcrm.DoAcceptSign(keyID, agreeResult, info.MsgHash, info.MsgContext)
			if err != nil {
				logWorkerError("accept", "accept sign job failed", err, "keyID", keyID, "result", res)
			} else {
				logWorker("accept", "accept sign job finish", "keyID", keyID, "result", agreeResult)
				addAcceptSignHistory(keyID, agreeResult, info.MsgHash, info.MsgContext)
			}
		}
		time.Sleep(waitInterval)
	}
}

func verifySignInfo(signInfo *dcrm.SignInfoData) error {
	if common.HexToAddress(signInfo.Account) != common.HexToAddress(params.GetServerDcrmUser()) {
		return errInitiatorMismatch
	}
	msgHash := signInfo.MsgHash
	msgContext := signInfo.MsgContext
	if len(msgContext) != 1 {
		return errWrongMsgContext
	}
	var args tokens.BuildTxArgs
	err := json.Unmarshal([]byte(msgContext[0]), &args)
	if err != nil {
		return errWrongMsgContext
	}
	switch args.Identifier {
	case params.GetIdentifier():
	case btc.AggregateIdentifier:
		if btc.BridgeInstance == nil {
			return tokens.ErrNoBtcBridge
		}
		logWorker("accept", "verifySignInfo", "msgHash", msgHash, "msgContext", msgContext)
		return btc.BridgeInstance.VerifyAggregateMsgHash(msgHash, &args)
	default:
		return errIdentifierMismatch
	}
	logWorker("accept", "verifySignInfo", "msgHash", msgHash, "msgContext", msgContext)
	return rebuildAndVerifyMsgHash(msgHash, &args)
}

func rebuildAndVerifyMsgHash(msgHash []string, args *tokens.BuildTxArgs) error {
	var (
		srcBridge, dstBridge tokens.CrossChainBridge
		memo                 string
	)
	switch args.SwapType {
	case tokens.SwapinType:
		srcBridge = tokens.SrcBridge
		dstBridge = tokens.DstBridge
	case tokens.SwapoutType:
		srcBridge = tokens.DstBridge
		dstBridge = tokens.SrcBridge
		memo = fmt.Sprintf("%s%s", tokens.UnlockMemoPrefix, args.SwapID)
	default:
		return fmt.Errorf("unknown swap type %v", args.SwapType)
	}
	var (
		swap *tokens.TxSwapInfo
		err  error
	)
	switch args.TxType {
	case tokens.P2shSwapinTx:
		if btc.BridgeInstance == nil {
			return tokens.ErrNoBtcBridge
		}
		swap, err = btc.BridgeInstance.VerifyP2shTransaction(args.SwapID, args.Bind, false)
	default:
		swap, err = srcBridge.VerifyTransaction(args.SwapID, false)
	}
	if err != nil {
		logWorkerError("accept", "verifySignInfo failed", err, "txid", args.SwapID, "swaptype", args.SwapType)
		return err
	}

	buildTxArgs := &tokens.BuildTxArgs{
		SwapInfo: args.SwapInfo,
		To:       swap.Bind,
		Value:    swap.Value,
		Memo:     memo,
		Extra:    args.Extra,
	}
	rawTx, err := dstBridge.BuildRawTransaction(buildTxArgs)
	if err != nil {
		return err
	}
	return dstBridge.VerifyMsgHash(rawTx, msgHash, args.Extra)
}

type acceptSignInfo struct {
	keyID      string
	result     string
	msgHash    []string
	msgContext []string
}

func addAcceptSignHistory(keyID, result string, msgHash, msgContext []string) {
	// Create the new item as its own ring
	item := ring.New(1)
	item.Value = &acceptSignInfo{
		keyID:      keyID,
		result:     result,
		msgHash:    msgHash,
		msgContext: msgContext,
	}

	acceptRingLock.Lock()
	defer acceptRingLock.Unlock()

	if acceptRing == nil {
		acceptRing = item
	} else {
		if acceptRing.Len() == acceptRingMaxSize {
			// Drop the block out of the ring
			acceptRing = acceptRing.Move(-1)
			acceptRing.Unlink(1)
			acceptRing = acceptRing.Move(1)
		}
		acceptRing.Move(-1).Link(item)
	}
}

func getAcceptSignHistory(keyID string) *acceptSignInfo {
	acceptRingLock.RLock()
	defer acceptRingLock.RUnlock()

	if acceptRing == nil {
		return nil
	}

	r := acceptRing
	for i := 0; i < r.Len(); i++ {
		item := r.Value.(*acceptSignInfo)
		if item.keyID == keyID {
			return item
		}
		r = r.Prev()
	}

	return nil
}
