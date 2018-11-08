# Using Docker For Deploying The Blockchain:

## Flow:
1. Code changes committed to git repo and triggers automatic docker image build process. Once build done image is pushed to Container Registry.
Build is performed by running `bridgez/docker/build.sh` and image is labeled with `:latest` label.
2. Deployment Engine (DE) triggered and shutdown instances, deploys or re-deploys the container and start them up.
3. Based on the instance type:
	1.  *[Note]*:
		1.  DE pulls instance-specific identity files (nodekey) and deploys into the container, at `/usr/eth/env/node`.
		*[Note]* To run test nodes rin: `./testNodes/testDeploy.sh node[1|2]` (1 or 2).
		2.  If no existing blockchain data exists - 
		run `./initNode.sh env/node`
			Else - grab the latest stored data folder and put into `/usr/eth/env/node1/geth/`.
		3. Start miner with `./startNode.sh`.
		*[Note]* This instance should be secured and accessible only to local network (other nodes).
	-----
	2.  **Bootnode**:
		1.  DE pulls bootnodekey into `/usr/eth/env/boot.key` and starts via `./startBootnode.sh env`.
		*[Note]* This instance is safe to be exposed to outer world.

		Optional params:
		1. `path`=`/usr/eth/env/node` | Path to the datadir containing the nodekey and chaindata.
		2. `port`=`39393` | Node's port
		3. `bootnode`=`127.0.0.1` | Bootnode's ip
		4. `rpcport`=`8545` | RPC port
		5. `rpcapi`=`personal,db,eth,net,web3,txpool,miner` | RPC api enabled
		6. `pswd`=`$path/password.txt` | Path to the file containing the password to unlock the account of this miner
	-----
	3.  **Client (RPC Proxy)**:
		1.  DE just runs `startClient.sh`, to start geth in safe mode. 
		*[Note]* This instance is safe to be exposed to outer world. 


