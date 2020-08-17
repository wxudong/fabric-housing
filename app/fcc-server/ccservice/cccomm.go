/*
Copyright xujf000@gmail.com .2020. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ccservice

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/pkg/errors"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/status"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"log"
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	fabAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	contextImpl "github.com/hyperledger/fabric-sdk-go/pkg/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
)

const (
	org1                 = "Org1"
	org2                 = "Org2"
	org3                 = "Org3"
	ordererAdminUser     = "Admin"
	ordererOrgName       = "OrdererOrg"
	org1AdminUser        = "Admin"
	org2AdminUser        = "Admin"
	org3AdminUser        = "Admin"
	org1User             = "User1"
	org2User             = "User1"
	org3User             = "User1"
	channelID            = "mychannel"
	goPath               = "/opt/gopath"
	ccFile               = "./config.yaml"
	orgchannelFile       = "./channel.tx"
	org1MSPanchorsFile   = "./Org1MSPanchors.tx"
	org2MSPanchorsFile   = "./Org2MSPanchors.tx"
	org3MSPanchorsFile   = "./Org3MSPanchors.tx"
	cc_netcon_ID         = "netcon"
	cc_estatebook_ID     = "estatebook"
	cc_estatetax_ID      = "estatetax"
	cc_netcon_Path       = "github.com/chaincode/netcon"
	cc_estatebook_Path   = "github.com/chaincode/estatebook"
	cc_estatetax_Path    = "github.com/chaincode/estatetax"
	ccVersion            = "0"
    ccNetcon             = "netcon"
	ccEstateBook         = "estatebook"
	ccEstatetax          = "estatetax"
)

var (
	// SDK
	sdk *fabsdk.FabricSDK
	cclient *channel.Client
	// Org MSP clients
	org1MspClient *mspclient.Client
	org2MspClient *mspclient.Client
	org3MspClient *mspclient.Client

	initArgs = [][]byte{}
)

// used to create context for different tests in the orgs package
type multiorgContext struct {
	// client contexts
	ordererClientContext   contextAPI.ClientProvider
	org1AdminClientContext contextAPI.ClientProvider
	org2AdminClientContext contextAPI.ClientProvider
	org3AdminClientContext contextAPI.ClientProvider
	org1ResMgmt            *resmgmt.Client
	org2ResMgmt            *resmgmt.Client
	org3ResMgmt            *resmgmt.Client
}

func setup() error {
	// Create SDK setup for the integration tests
	var err error
	sdk, err = fabsdk.New(config.FromFile(ccFile))
	if err != nil {
		log.Println("Failed to create new SDK, err : %s", err)
		return err
	}

	org1MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org1))
	if err != nil {
		log.Println("failed to create org1MspClient, err : %s", err)
		return err
	}

	org2MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org2))
	if err != nil {
		log.Println("failed to create org2MspClient, err : %s", err)
		return err
	}

	org3MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org3))
	if err != nil {
		log.Println("failed to create org3MspClient, err : %s", err)
		return err
	}

	return nil
}

func Init() {

	err := setup()
	if err != nil {
		log.Println("unable to setup [%s]", err)
	}

	//prepare contexts
	mc := multiorgContext{
		ordererClientContext:   sdk.Context(fabsdk.WithUser(ordererAdminUser), fabsdk.WithOrg(ordererOrgName)),
		org1AdminClientContext: sdk.Context(fabsdk.WithUser(org1AdminUser), fabsdk.WithOrg(org1)),
		org2AdminClientContext: sdk.Context(fabsdk.WithUser(org2AdminUser), fabsdk.WithOrg(org2)),
		org3AdminClientContext: sdk.Context(fabsdk.WithUser(org3AdminUser), fabsdk.WithOrg(org3)),
	}

	setupClientContextsAndChannel(sdk, &mc)

	createAndJoinChannel(&mc)

	createAndInstantiateCC(&mc)

	InitCCOnStart()

}

func setupClientContextsAndChannel(sdk *fabsdk.FabricSDK, mc *multiorgContext) {
	// Org1 resource management client (Org1 is default org)
	org1RMgmt, err := resmgmt.New(mc.org1AdminClientContext)
	if err != nil {
		log.Println("failed to create org1 resource management client, err : %s", err)
	}

	mc.org1ResMgmt = org1RMgmt

	// Org2 resource management client
	org2RMgmt, err := resmgmt.New(mc.org2AdminClientContext)
	if err != nil {
		log.Println("failed to create org2 resource management client, err : %s", err)
	}

	mc.org2ResMgmt = org2RMgmt

	// Org3 resource management client
	org3RMgmt, err := resmgmt.New(mc.org3AdminClientContext)
	if err != nil {
		log.Println("failed to create org2 resource management client, err : %s", err)
	}

	mc.org3ResMgmt = org3RMgmt
}

func createAndJoinChannel(mc *multiorgContext) {
	// Get signing identity that is used to sign create channel request
	org1AdminUser, err := org1MspClient.GetSigningIdentity(org1AdminUser)
	if err != nil {
		log.Println("failed to get org1AdminUser, err : %s", err)
	}

	org2AdminUser, err := org2MspClient.GetSigningIdentity(org2AdminUser)
	if err != nil {
		log.Println("failed to get org2AdminUser, err : %s", err)
	}

	org3AdminUser, err := org3MspClient.GetSigningIdentity(org3AdminUser)
	if err != nil {
		log.Println("failed to get org3AdminUser, err : %s", err)
	}

	createChannel(org1AdminUser, org2AdminUser, org3AdminUser, mc)
	
	log.Println("Org1 JoinChannel")
	err = mc.org1ResMgmt.JoinChannel(channelID, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("Org1 peers failed to JoinChannel, err : %s", err)
	}

	log.Println("Org2 JoinChannel")
	err = mc.org2ResMgmt.JoinChannel(channelID, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("Org2 peers failed to JoinChannel, err : %s", err)
	}

	log.Println("Org3 JoinChannel")
	err = mc.org3ResMgmt.JoinChannel(channelID, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("Org3 peers failed to JoinChannel, err : %s", err)
	}
}

func createChannel(org1AdminUser msp.SigningIdentity, org2AdminUser msp.SigningIdentity, org3AdminUser msp.SigningIdentity, mc *multiorgContext) {
	// Channel management client is responsible for managing channels (create/update channel)
	log.Println("Creating channel...")
	chMgmtClient, err := resmgmt.New(mc.ordererClientContext)
	if err != nil {
		log.Println("failed to get a new channel management client, err : %s", err)
	}
	// create a channel for orgchannel.tx
	req := resmgmt.SaveChannelRequest{ChannelID: channelID,
		ChannelConfigPath: orgchannelFile,
		SigningIdentities: []msp.SigningIdentity{org1AdminUser, org2AdminUser, org3AdminUser}}
	_, err = chMgmtClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("error should be nil for SaveChannel of orgchannel, err : %s", err)
	}

	log.Println("Updating anchor peers for org1...")
	chMgmtClient, err = resmgmt.New(mc.org1AdminClientContext)
	req = resmgmt.SaveChannelRequest{ChannelID: channelID,
		ChannelConfigPath: "./Org1MSPanchors.tx",
		SigningIdentities: []msp.SigningIdentity{org1AdminUser}}
	_, err = chMgmtClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("error should be nil for SaveChannel for anchor peer 1, err : %s", err)
	}

	log.Println("Updating anchor peers for org2...")
	chMgmtClient, err = resmgmt.New(mc.org2AdminClientContext)
	req = resmgmt.SaveChannelRequest{ChannelID: channelID,
		ChannelConfigPath: "./Org2MSPanchors.tx",
		SigningIdentities: []msp.SigningIdentity{org2AdminUser}}
	_, err = chMgmtClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("error should be nil for SaveChannel for anchor peer 2, err : %s", err)
	}

	log.Println("Updating anchor peers for org3...")
	chMgmtClient, err = resmgmt.New(mc.org3AdminClientContext)
	req = resmgmt.SaveChannelRequest{ChannelID: channelID,
		ChannelConfigPath: "./Org3MSPanchors.tx",
		SigningIdentities: []msp.SigningIdentity{org3AdminUser}}
	_, err = chMgmtClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint("orderer.example.com"))
	if err != nil {
		log.Println("error should be nil for SaveChannel for anchor peer 3, err : %s", err)
	}
}

func createAndInstantiateCC(mc *multiorgContext) error {
	cc_netcon_Pkg, err := packager.NewCCPackage(cc_netcon_Path, goPath)
	if err != nil {
		log.Println("WARN: createCC: ", err.Error())
		return err
	}

	cc_estatebook_Pkg, err := packager.NewCCPackage(cc_estatebook_Path, goPath)
	if err != nil {
		log.Println("WARN: createCC: ", err.Error())
		return err
	}

	cc_estatetax_Pkg, err := packager.NewCCPackage(cc_estatetax_Path, goPath)
	if err != nil {
		log.Println("WARN: createCC: ", err.Error())
		return err
	}

	/*org1Peers, err := DiscoverLocalPeers(mc.org1AdminClientContext, 2)
	if err != nil {
		log.Println("DiscoverLocalPeers: ", err.Error())
		return err
	}
	org2Peers, err := DiscoverLocalPeers(mc.org2AdminClientContext, 2)
	if err != nil {
		log.Println("DiscoverLocalPeers: ", err.Error())
		return err
	}
	org3Peers, err := DiscoverLocalPeers(mc.org3AdminClientContext, 2)
	if err != nil {
		log.Println("DiscoverLocalPeers: ", err.Error())
		return err
	}*/

    log.Println("InstallCC netcon on Org1/Org2/Org3...")
	installCCReq := resmgmt.InstallCCRequest{Name: cc_netcon_ID, Path: cc_netcon_Path, Version: ccVersion, Package: cc_netcon_Pkg}
	_, err = mc.org2ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org2 InstallCC: ", err.Error())
		return err
	}
	_, err = mc.org3ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org3 InstallCC: ", err.Error())
		return err
	}

	_, err = mc.org1ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org1 InstallCC: ", err.Error())
		return err
	}

	log.Println("InstallCC estatebook on Org1/Org2/Org3...")
	installCCReq = resmgmt.InstallCCRequest{Name: cc_estatebook_ID, Path: cc_estatebook_Path, Version: ccVersion, Package: cc_estatebook_Pkg}
	_, err = mc.org1ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org1 InstallCC: ", err.Error())
		return err
	}
	_, err = mc.org2ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org2 InstallCC: ", err.Error())
		return err
	}
	_, err = mc.org3ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org3 InstallCC: ", err.Error())
		return err
	}

	log.Println("InstallCC estatetax on Org1/Org2/Org3...")
	installCCReq = resmgmt.InstallCCRequest{Name: cc_estatetax_ID, Path: cc_estatetax_Path, Version: ccVersion, Package: cc_estatetax_Pkg}
	_, err = mc.org1ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org1 InstallCC: ", err.Error())
		return err
	}
	_, err = mc.org2ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org2 InstallCC: ", err.Error())
		return err
	}
	_, err = mc.org3ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Println("WARN: Org3 InstallCC: ", err.Error())
		return err
	}

	

/*	installed := queryInstalledCC("Org1", mc.org1ResMgmt, cc_netcon_ID, ccVersion, org1Peers)
	log.Println("Expecting chaincode to be installed on all peers in Org1", installed)
	time.Sleep(time.Second*5)

	installed = queryInstalledCC("Org2", mc.org2ResMgmt, cc_netcon_ID, ccVersion, org2Peers)
	log.Println("Expecting chaincode to be installed on all peers in Org2", installed)
	time.Sleep(time.Second*5)

	installed = queryInstalledCC("Org3", mc.org3ResMgmt, cc_netcon_ID, ccVersion, org3Peers)
	log.Println("Expecting chaincode to be installed on all peers in Org3", installed)
	time.Sleep(time.Second*5)*/

	log.Println("Instantiating netcon chaincode on Org1")
	_, err = instantiateChaincode(mc.org1ResMgmt, channelID, cc_netcon_ID, cc_netcon_Path, ccVersion, "OR ('Org1MSP.member','Org2MSP.member','Org3MSP.member')", initArgs)
	if err != nil {
		log.Println("transaction response should be populated ", err.Error())
		return err
	}	

	log.Println("Instantiating estatebook chaincode on Org2")
	_, err = instantiateChaincode(mc.org2ResMgmt, channelID, cc_estatebook_ID, cc_estatebook_Path, ccVersion, "OR ('Org1MSP.member','Org2MSP.member','Org3MSP.member')", initArgs)
	if err != nil {
		log.Println("transaction response should be populated ", err.Error())
		return err
	}

	log.Println("Instantiating estatetax chaincode on Org3")
	_, err = instantiateChaincode(mc.org3ResMgmt, channelID, cc_estatetax_ID, cc_estatetax_Path, ccVersion, "AND ('Org1MSP.member','Org3MSP.member','Org3MSP.member')", initArgs)
	if err != nil {
		log.Println("transaction response should be populated ", err.Error())
		return err
	}


/*	found := queryInstantiatedCC("Org1", mc.org1ResMgmt, channelID, cc_netcon_ID, ccVersion, org1Peers)
	log.Println("Expecting chaincode to be instantiate on all peers in Org1", found)
	time.Sleep(time.Second*5)

	found = queryInstantiatedCC("Org2", mc.org2ResMgmt, channelID, cc_netcon_ID, ccVersion, org2Peers)
	log.Println("Expecting chaincode to be instantiate on all peers in Org3", found)
	time.Sleep(time.Second*5)

	found = queryInstantiatedCC("Org3", mc.org3ResMgmt, channelID, cc_netcon_ID, ccVersion, org3Peers)
	log.Println("Expecting chaincode to be instantiate on all peers in Org3", found)
	time.Sleep(time.Second*5)*/

	/*found = queryInstantiatedCC("Org2", mc.org1ResMgmt, channelID, cc_estatebook_ID, ccVersion, org2Peers)
	log.Println("Expecting chaincode to be instantiate on all peers in Org2", found)
	time.Sleep(time.Second*5)

	found = queryInstantiatedCC("Org3", mc.org1ResMgmt, channelID, cc_estatetax_ID, ccVersion, org3Peers)
	log.Println("Expecting chaincode to be instantiate on all peers in Org3", found)*/

	return nil
}

// InstantiateChaincode instantiates the given chaincode to the given channel
func instantiateChaincode(resMgmt *resmgmt.Client, channelID, ccName, ccPath, ccVersion string, ccPolicyStr string, args [][]byte) (resmgmt.InstantiateCCResponse, error) {
	ccPolicy, err := cauthdsl.FromString(ccPolicyStr)
	if err != nil {
		return resmgmt.InstantiateCCResponse{}, errors.Wrapf(err, "error creating CC policy [%s]", ccPolicyStr)
	}

	return resMgmt.InstantiateCC(
		channelID,
		resmgmt.InstantiateCCRequest{
			Name:       ccName,
			Path:       ccPath,
			Version:    ccVersion,
			Args:       args,
			Policy:     ccPolicy,
		},
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
	)
}

func DiscoverLocalPeers(ctxProvider contextAPI.ClientProvider, expectedPeers int) ([]fabAPI.Peer, error) {
	ctx, err := contextImpl.NewLocal(ctxProvider)
	if err != nil {
		return nil, errors.Wrap(err, "error creating local context")
	}

	discoveredPeers, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			peers, serviceErr := ctx.LocalDiscoveryService().GetPeers()
			if serviceErr != nil {
				return nil, errors.Wrapf(serviceErr, "error getting peers for MSP [%s]", ctx.Identifier().MSPID)
			}
			if len(peers) < expectedPeers {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Expecting %d peers but got %d", expectedPeers, len(peers)), nil)
			}
			return peers, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return discoveredPeers.([]fabAPI.Peer), nil
}

func queryInstalledCC(orgID string, resMgmt *resmgmt.Client, ccName, ccVersion string, peers []fab.Peer) bool {
	installed, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			ok := isCCInstalled(orgID, resMgmt, ccName, ccVersion, peers)
			if !ok {
				return &ok, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Chaincode [%s:%s] is not installed on all peers in Org", ccName, ccVersion), nil)
			}
			return &ok, nil
		},
	)
	if err != nil {
		log.Println("Got error checking if chaincode was installed", err.Error())
	}
	return *(installed).(*bool)
}

func isCCInstalled(orgID string, resMgmt *resmgmt.Client, ccName, ccVersion string, peers []fab.Peer) bool {
	log.Println("Querying peers to see if chaincode was installed", orgID, ccName, ccVersion)
	installedOnAllPeers := true
	for _, peer := range peers {
		log.Println("Querying [%s] ...", peer.URL())
		resp, err := resMgmt.QueryInstalledChaincodes(resmgmt.WithTargets(peer))
		if(err != nil){
			log.Println("QueryInstalledChaincodes for peer failed", peer.URL())
		}

		found := false
		for _, ccInfo := range resp.Chaincodes {
			log.Println("found chaincode ", ccInfo.Name, ccInfo.Version)
			if ccInfo.Name == ccName && ccInfo.Version == ccVersion {
				found = true
				break
			}
		}
		if !found {
			log.Println("chaincode is not installed on peer ", ccName, ccVersion, peer.URL())
			installedOnAllPeers = false
		}
	}
	return installedOnAllPeers
}

func queryInstantiatedCC(orgID string, resMgmt *resmgmt.Client, channelID, ccName, ccVersion string, peers []fab.Peer) bool {
	log.Println("Querying peers to see if chaincode was instantiated on channel ", orgID, ccName, channelID)

	instantiated, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			ok := isCCInstantiated(resMgmt, channelID, ccName, ccVersion, peers)
			if !ok {
				return &ok, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Did NOT find instantiated chaincode [%s:%s] on one or more peers in [%s].", ccName, ccVersion, orgID), nil)
			}
			return &ok, nil
		},
	)
	if err != nil {
		log.Println("Got error checking if chaincode was instantiated", err.Error())
	}
	return *(instantiated).(*bool)
}

func isCCInstantiated(resMgmt *resmgmt.Client, channelID, ccName, ccVersion string, peers []fab.Peer) bool {
	installedOnAllPeers := true
	for _, peer := range peers {
		log.Println("Querying peer for instantiated chaincode ", peer.URL(), ccName, ccVersion)
		chaincodeQueryResponse, err := resMgmt.QueryInstantiatedChaincodes(channelID, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithTargets(peer))
		if err != nil {
			log.Println("QueryInstantiatedChaincodes return error", err.Error())
		}
		log.Println("Found instantiated chaincodes on peer ", len(chaincodeQueryResponse.Chaincodes), peer.URL())
		found := false
		for _, chaincode := range chaincodeQueryResponse.Chaincodes {
			log.Println("Found instantiated chaincode Name, ", chaincode.Name, chaincode.Version, chaincode.Path, peer.URL())
			if chaincode.Name == ccName && chaincode.Version == ccVersion {
				found = true
				break
			}
		}
		if !found {
			log.Println("chaincode is not instantiated on peer ", ccName, ccVersion, peer.URL())
			installedOnAllPeers = false
		}
	}
	return installedOnAllPeers
}

func InitCCOnStart() {
	var err error
	clientContext := sdk.ChannelContext(channelID, fabsdk.WithUser(org1User), fabsdk.WithOrg(org1))
	if clientContext == nil {
		log.Println("WARN: init Chaincode clientContext error:", err.Error())
	} else {
		cclient, err = channel.New(clientContext)
		if err != nil {
			log.Println("WARN: init Chaincode cclient error:", err.Error())
		}
	}
	log.Println("Chaincode client initialed successfully.")
}

func GetChannelClient() *channel.Client {
	return cclient
}

func CCinvoke(channelClient *channel.Client, ccname, fcn string, args []string) ([]byte, error) {
	var tempArgs [][]byte
	for i := 0; i < len(args); i++ {
		tempArgs = append(tempArgs, []byte(args[i]))
	}
	qrequest := channel.Request{
		ChaincodeID:     ccname,
		Fcn:             fcn,
		Args:            tempArgs,
		TransientMap:    nil,
		InvocationChain: nil,
	}
	//log.Println("cc exec request:",qrequest.ChaincodeID,"\t",qrequest.Fcn,"\t",qrequest.Args)
	response, err := channelClient.Execute(qrequest)
	if err != nil {
		return nil, err
	}
	return response.Payload, nil
}

func CCquery(channelClient *channel.Client, ccname, fcn string, args []string) ([]byte, error) {
	var tempArgs [][]byte
	if args == nil {
		tempArgs = nil
	} else {
		for i := 0; i < len(args); i++ {
			tempArgs = append(tempArgs, []byte(args[i]))
		}
	}
	qrequest := channel.Request{
		ChaincodeID:     ccname,
		Fcn:             fcn,
		Args:            tempArgs,
		TransientMap:    nil,
		InvocationChain: nil,
	}
	response, err := channelClient.Query(qrequest)
	if err != nil {
		return nil, err
	}
	return response.Payload, nil
}
