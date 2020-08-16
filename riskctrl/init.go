package riskctrl

import (
	"github.com/anyswap/CrossChain-Bridge/log"
	"github.com/anyswap/CrossChain-Bridge/tokens"
	"github.com/anyswap/CrossChain-Bridge/tokens/bridge"
	"github.com/anyswap/CrossChain-Bridge/tools"
)

var (
	srcBridge tokens.CrossChainBridge
	dstBridge tokens.CrossChainBridge
)

// InitCrossChainBridge init bridge
func InitCrossChainBridge() {
	cfg := GetConfig()
	srcToken := cfg.SrcToken
	dstToken := cfg.DestToken
	srcGateway := cfg.SrcGateway
	dstGateway := cfg.DestGateway

	srcID := srcToken.BlockChain
	dstID := dstToken.BlockChain
	srcNet := srcToken.NetID
	dstNet := dstToken.NetID

	srcBridge = bridge.NewCrossChainBridge(srcID, true)
	dstBridge = bridge.NewCrossChainBridge(dstID, false)
	log.Info("New bridge finished", "source", srcID, "sourceNet", srcNet, "dest", dstID, "destNet", dstNet)

	srcBridge.SetTokenAndGateway(srcToken, srcGateway, false)
	log.Info("Init bridge source", "token", srcToken.Symbol, "gateway", srcGateway)

	dstBridge.SetTokenAndGateway(dstToken, dstGateway, false)
	log.Info("Init bridge destation", "token", dstToken.Symbol, "gateway", dstGateway)
}

// InitEmailConfig init email config
func InitEmailConfig() {
	if riskConfig.Email == nil {
		log.Info("no email is config, ignore it")
		return
	}
	server := riskConfig.Email.Server
	port := riskConfig.Email.Port
	from := riskConfig.Email.From
	name := riskConfig.Email.FromName
	password := riskConfig.Email.Password
	tools.InitEmailConfig(server, port, from, name, password)
	log.Info("init email config", "server", server, "port", port, "from", from, "name", name)
}
