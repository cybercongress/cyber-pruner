package cmd

import (
	"encoding/binary"
	"fmt"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tendermint/tendermint/state"
	tmstore "github.com/tendermint/tendermint/store"
	db "github.com/tendermint/tm-db"

	"github.com/cybercongress/cyber-pruner/internal/rootmulti"
)

// to figuring out the height to prune tx_index
var txIdxHeight int64 = 0

type pruningProfile struct {
	name         string
	blocks       uint64
	keepVersions uint64
	keepEvery    uint64
}

var (
	PruningProfiles = map[string]pruningProfile{
		"default":    {"default", 0, 400000, 100},
		"nothing":    {"nothing", 0, 0, 1},
		"everything": {"everything", 0, 10, 0},
		"emitter":    {"emitter", 320000, 100, 0},
		"rest-light": {"rest-light", 640000, 100000, 0},
		"rest-heavy": {"rest-heavy", 0, 400000, 1000},
		"peer":       {"peer", 0, 100, 30000},
		"seed":       {"seed", 320000, 100, 0},
		"sentry":     {"sentry", 640000, 100, 0},
		"validator":  {"validator", 320000, 100, 0},
	}
)

// load db
// load app store and prune
// if immutable tree is not deletable we should import and export current state
func pruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "prune data from the application store and block store",
		RunE: func(cmd *cobra.Command, args []string) error {

			if profile != "custom" {
				if _, ok := PruningProfiles[profile]; !ok {
					return fmt.Errorf("Invalid Pruning Profile")
				}
				if !cmd.Flag("min-retain-blocks").Changed && cmd.Flag("pruning").Changed {
					blocks = PruningProfiles[profile].blocks
				}
				if !cmd.Flag("pruning-keep-recent").Changed {
					keepVersions = PruningProfiles[profile].keepVersions
				}
				if !cmd.Flag("pruning-keep-every").Changed {
					keepEvery = PruningProfiles[profile].keepEvery
				}
			}

			//fmt.Println("app:", app)
			fmt.Println("profile:", profile)
			fmt.Println("pruning-keep-every:", keepEvery)
			fmt.Println("pruning-keep-recent:", keepVersions)
			fmt.Println("min-retain-blocks:", blocks)
			fmt.Println("batch:", batch)
			fmt.Println("parallel-limit:", parallel)

			var err error
			if cosmosSdk {
				if err = pruneAppState(homePath); err != nil {
					return err
				}
			}

			if tendermint {
				if err = pruneTMData(homePath); err != nil {
					return err
				}
			}

			if txIndex {
				if err = pruneTxIndex(homePath); err != nil {
					return err
				}
			}

			return nil
		},
	}
	return cmd
}

func compactCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "compact",
		Short: "compact data from the application store and block store",
		RunE: func(cmd *cobra.Command, args []string) error {

			if cosmosSdk {
				appDB, errDB := openDB("application", homePath)
				if errDB != nil {
					return errDB
				}

				fmt.Println("[~] compacting application state")
				if err := compactDB(appDB); err != nil {
					fmt.Println(err.Error())
				} else {
					fmt.Println("[+] finished compacting application state")
				}
				appDB.Close()
			}

			if tendermint {
				blockStoreDB, errDB := openDB("blockstore", homePath)
				if errDB != nil {
					return errDB
				}

				fmt.Println("[~] compacting block store")
				if err := compactDB(blockStoreDB); err != nil {
					fmt.Println(err.Error())
				} else {
					fmt.Println("[+] finished compacting block store")
				}
				blockStoreDB.Close()

				stateDB, errDB := openDB("state", homePath)
				if errDB != nil {
					return errDB
				}

				fmt.Println("[~] compacting state store")
				if err := compactDB(stateDB); err != nil {
					fmt.Println(err.Error())
				} else {
					fmt.Println("[+] finished compacting state store")
				}
				stateDB.Close()
			}

			if txIndex {
				txIdxDB, err := openDB("tx_index", homePath)
				if err != nil {
					return err
				}

				fmt.Println("[~] compacting tx_index")
				if err := compactDB(txIdxDB); err != nil {
					fmt.Println(err.Error())
				} else {
					fmt.Println("[+] finished compacting tx_index")
				}
				txIdxDB.Close()
			}

			return nil
		},
	}

	return cmd
}

func pruneAppState(home string) error {

	appDB, errDB := openDB("application", home)
	if errDB != nil {
		return errDB
	}

	defer appDB.Close()

	var err error

	fmt.Println("[~] pruning application state")

	keys := getStoreKeys(appDB)

	wg := sync.WaitGroup{}
	var prune_err error

	guard := make(chan struct{}, parallel)
	for _, value := range keys {
		guard <- struct{}{}
		wg.Add(1)
		go func(value string) {
			err := func(value string) error {
				appStore := rootmulti.NewStore(appDB)
				appStore.MountStoreWithDB(storetypes.NewKVStoreKey(value), sdk.StoreTypeIAVL, nil)
				err = appStore.LoadLatestVersion()
				if err != nil {
					return err
				}

				versions := appStore.GetAllVersions()

				v64 := make([]int64, 0)
				for i := 0; i < len(versions); i++ {
					if (keepEvery == 0 || versions[i]%int(keepEvery) != 0) &&
						versions[i] <= versions[len(versions)-1]-int(keepVersions) {
						v64 = append(v64, int64(versions[i]))
					}
				}

				appStore.PruneHeights = v64[:]

				fmt.Printf("[~] pruning store: %+v (%d/%d)\n", value, len(v64), len(versions))
				appStore.PruneStores(int(batch))
				fmt.Println("[+] finished pruning store:", value)

				return nil
			}(value)

			if err != nil {
				prune_err = err
			}
			<-guard
			defer wg.Done()
		}(value)
	}
	wg.Wait()

	if prune_err != nil {
		return prune_err
	}
	fmt.Println("[+] finished pruning application state")

	if compact {
		fmt.Println("[~] compacting application state")
		if err := compactDB(appDB); err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("[+] finished compacting application state")
		}
	}

	return nil
}

// pruneTMData prunes the tendermint blocks and state based on the amount of blocks to keep
func pruneTMData(home string) error {

	blockStoreDB, errDBBlock := openDB("blockstore", home)
	if errDBBlock != nil {
		return errDBBlock
	}

	blockStore := tmstore.NewBlockStore(blockStoreDB)
	defer blockStore.Close()

	// Get StateStore
	stateDB, errDBBState := openDB("state", home)
	if errDBBState != nil {
		return errDBBState
	}

	var err error

	stateStore := state.NewStore(stateDB)
	defer stateStore.Close()

	base := blockStore.Base()

	if blocks == 0 {
		return nil
	}
	if blocks < 100000 {
		return fmt.Errorf("Your min-retain-blocks %+v is lower than the minimum 100000", blocks)
	}

	pruneHeight := blockStore.Height() - int64(blocks)

	fmt.Println("[~] pruning block store")
	// prune block store
	// prune one by one instead of range to avoid `panic: pebble: batch too large: >= 4.0 G` issue
	// (see https://github.com/notional-labs/cosmprund/issues/11)
	for pruneBlockFrom := base; pruneBlockFrom < pruneHeight-1; pruneBlockFrom += int64(batch) {
		height := pruneBlockFrom
		if height >= pruneHeight-1 {
			height = pruneHeight - 1
		}

		_, err = blockStore.PruneBlocks(height)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	fmt.Println("[+] finished pruning block store")

	if compact {
		fmt.Println("[~] compacting block store")
		if err := compactDB(blockStoreDB); err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("[+] finished compacting block store")
		}
	}

	fmt.Println("[~] pruning state store")
	// prune state store
	// prune one by one instead of range to avoid `panic: pebble: batch too large: >= 4.0 G` issue
	// (see https://github.com/notional-labs/cosmprund/issues/11)
	for pruneStateFrom := base; pruneStateFrom < pruneHeight-1; pruneStateFrom += int64(batch) {
		endHeight := pruneStateFrom + int64(batch)
		if endHeight >= pruneHeight-1 {
			endHeight = pruneHeight - 1
		}
		err = stateStore.PruneStates(pruneStateFrom, endHeight)
		if err != nil {
			//return err
			fmt.Println(err.Error())
		}
	}
	fmt.Println("[+] finished pruning state store")

	if compact {
		fmt.Println("[~] compacting state store")
		if err := compactDB(stateDB); err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("[+] finished compacting state store")
		}
	}

	return nil
}

func pruneTxIndex(home string) error {
	fmt.Println("[~] pruning tx_index")
	txIdxDB, err := openDB("tx_index", home)
	if err != nil {
		return err
	}

	defer func() {
		errClose := txIdxDB.Close()
		if errClose != nil {
			fmt.Println(errClose.Error())
		}
	}()

	pruneHeight := txIdxHeight - int64(blocks) - 10
	if pruneHeight <= 0 {
		fmt.Printf("No need to prune (pruneHeight=%d)\n", pruneHeight)
		return nil
	}

	pruneBlockIndex(txIdxDB, pruneHeight)
	pruneTxIndexTxs(txIdxDB, pruneHeight)

	fmt.Println("[+] finished pruning tx_index")

	if compact {
		fmt.Println("[~] compacting tx_index")
		if err := txIdxDB.ForceCompact(nil, nil); err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("[+] finished compacting tx_index")
		}
	}

	return nil
}

func pruneTxIndexTxs(db db.DB, pruneHeight int64) {
	itr, itrErr := db.Iterator(nil, nil)
	if itrErr != nil {
		panic(itrErr)
	}

	defer itr.Close()

	///////////////////////////////////////////////////
	// delete index by hash and index by height
	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		value := itr.Value()

		strKey := string(key)

		if strings.HasPrefix(strKey, "tx.height") { // index by height
			strs := strings.Split(strKey, "/")
			intHeight, _ := strconv.ParseInt(strs[2], 10, 64)

			if intHeight < pruneHeight {
				db.Delete(value)
				db.Delete(key)
			}
		} else {
			if len(value) == 32 { // maybe index tx by events
				strs := strings.Split(strKey, "/")
				if len(strs) == 4 { // index tx by events
					intHeight, _ := strconv.ParseInt(strs[2], 10, 64)
					if intHeight < pruneHeight {
						db.Delete(key)
					}
				}
			}
		}
	}
}

func pruneBlockIndex(db db.DB, pruneHeight int64) {
	itr, itrErr := db.Iterator(nil, nil)
	if itrErr != nil {
		panic(itrErr)
	}

	defer itr.Close()

	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		value := itr.Value()

		strKey := string(key)

		if strings.HasPrefix(strKey, "block.height") /* index block primary key*/ || strings.HasPrefix(strKey, "block_events") /* BeginBlock & EndBlock */ {
			intHeight := int64FromBytes(value)
			//fmt.Printf("intHeight: %d\n", intHeight)

			if intHeight < pruneHeight {
				db.Delete(key)
			}
		}
	}
}

func openDB(dbname string, home string) (db.DB, error) {
	dbType := db.BackendType(backend)
	dbDir := rootify(dataDir, home)

	var db1 db.DB

	if dbType == db.GoLevelDBBackend {
		o := opt.Options{
			DisableSeeksCompaction: true,
		}

		lvlDB, err := db.NewGoLevelDBWithOpts(dbname, dbDir, &o)
		if err != nil {
			return nil, err
		}

		db1 = lvlDB
	} else {
		panic("not implemented")
	}

	return db1, nil
}

func compactDB(vdb db.DB) error {
	dbType := db.BackendType(backend)

	if dbType == db.GoLevelDBBackend {
		vdbLevel := vdb.(*db.GoLevelDB)

		if err := vdbLevel.ForceCompact(nil, nil); err != nil {
			return err
		}
	} else {
		panic("not implemented")
	}

	return nil
}

func getStoreKeys(db db.DB) (storeKeys []string) {
	latestVer := rootmulti.GetLatestVersion(db)
	latestCommitInfo, err := getCommitInfo(db, latestVer)
	if err != nil {
		panic(err)
	}

	for _, storeInfo := range latestCommitInfo.StoreInfos {
		storeKeys = append(storeKeys, storeInfo.Name)
	}
	return
}

func getCommitInfo(db db.DB, ver int64) (*storetypes.CommitInfo, error) {
	const commitInfoKeyFmt = "s/%d" // s/<version>
	cInfoKey := fmt.Sprintf(commitInfoKeyFmt, ver)

	bz, err := db.Get([]byte(cInfoKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get commit info: %s", err)
	} else if bz == nil {
		return nil, fmt.Errorf("no commit info found")
	}

	cInfo := &storetypes.CommitInfo{}
	if err = cInfo.Unmarshal(bz); err != nil {
		return nil, fmt.Errorf("failed unmarshal commit info: %s", err)
	}

	return cInfo, nil
}

// Utils
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func int64FromBytes(bz []byte) int64 {
	v, _ := binary.Varint(bz)
	return v
}
