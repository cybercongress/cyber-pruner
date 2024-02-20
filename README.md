# cyber-pruner

```
# clone & build cosmprund repo
git clone https://github.com/cybercongress/cyber-pruner.git
cd cyber-pruner
make install

# stop daemon/cosmovisor
sudo systemctl stop cyber

# run pruning using config from app.toml
cyber-pruner prune

# run compacting
cyber-pruner compact

# run pruning with params
cyber-pruner prune --home ~/.cyber --pruning validator

# run compacting with params
cyber-pruner compact --home ~/.cyber


```

> Note: Application pruning can take a very long time dependent on the size of the db.

Flags: 

- `home`: path to directory for config and data (default=~/.cyber)
- `cosmos-sdk`: If pruning a non cosmos-sdk chain, like Nomic, you only want to use tendermint pruning or if you want to only prune tendermint block & state as this is generally large on machines(Default true)
- `tendermint`: If the user wants to only prune application data they can disable pruning of tendermint data. (Default true)
- `tx-index`: If the user wants to prune the transaction and block events index (Default false)
- `min-retain-blocks`: set the amount of tendermint blocks to be kept (default=300000)
- `pruning-keep-recent`: set the amount of versions to keep in the application store (default=500000)
- `pruning-keep-every`: set the version interval to be kept in the application store (default=None)
- `pruning`: pruning profile (default "default")
- `batch`: set the amount of versions to be pruned in one batch (default=10000)
- `parallel-limit`: set the limit of parallel go routines to be running at the same time (default=16)
- `compact`: compact the stores after pruning (default=true)
  
#### Pruning profiles
- **default** 
  - min-retain-blocks : 0
  - pruning-keep-recent: 400000
  - pruning-keep-every: 100
- **everything** 
  - min-retain-blocks : 0
  - pruning-keep-recent: 10
  - pruning-keep-every: 0
- **nothing** 
  - min-retain-blocks : 0
  - pruning-keep-recent: 0
  - pruning-keep-every: 1
- **emitter** 
  - min-retain-blocks : 320000
  - pruning-keep-recent: 100
  - pruning-keep-every: None
- **rest-light** 
  - min-retain-blocks : 640000
  - pruning-keep-recent: 100000
  - pruning-keep-every: None
- **rest-heavy** 
  - min-retain-blocks : Keep all
  - pruning-keep-recent: 400000
  - pruning-keep-every: 1000
- **peer** 
  - min-retain-blocks : Keep all
  - pruning-keep-recent: 100
  - pruning-keep-every: 30000
- **seed** 
  - min-retain-blocks : 320000
  - pruning-keep-recent: 100
  - pruning-keep-every: None
- **sentry** 
  - min-retain-blocks : 640000
  - pruning-keep-recent: 100
  - pruning-keep-every: None
- **validator** 
  - min-retain-blocks : 320000
  - pruning-keep-recent: 100
  - pruning-keep-every: None

## Acknowledgments
- [binaryholdings](https://github.com/binaryholdings/cosmprund)
- [bandprotocol](https://github.com/bandprotocol/cosmprund)
- [notional-labs](https://github.dev/notional-labs/cosmprund)
- and all contributors and node operators who have been testing and providing feedback on the pruning process
