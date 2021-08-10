# PublicBlockChain_FUU
# FUU基于的公链实现(具备公链全功能,私有化公链)

// 采用TCP
const PROTOCOL  = "tcp"
// 發送消息的前12個字節指定了命令名(version)
const COMMANDLENGTH  = 12
// 節點的區塊鏈版本
const NODE_VERSION  = 1

// 命令
// 版本命令
const COMMAND_VERSION  = "version"
const COMMAND_ADDR  = "addr"
const COMMAND_BLOCK  = "block"
const COMMAND_INV  = "inv"
const COMMAND_GETBLOCKS  = "getblocks"
const COMMAND_GETDATA  = "getdata"
const COMMAND_TX  = "tx"

// 類型
const BLOCK_TYPE  = "block"
const TX_TYPE  = "tx"
```



## Version

Version消息是發起區塊同步第一個發送的消息類型，其內容主要有區塊鏈版本，區塊鏈最大高度，來自的節點地址。它主要用於比較兩個節點間誰是最長鏈。

```
type Version struct {
	// 區塊鏈版本
	Version    int64
	// 請求節點區塊的高度
	BestHeight int64
	// 請求節點的地址
	AddrFrom   string
}
```

組裝發送Version信息

```
//發送COMMAND_VERSION
func sendVersion(toAddress string, fuu *Blockchain)  {


	bestHeight := fuu.GetBestHeight()
	payload := gobEncode(Version{NODE_VERSION, bestHeight, nodeAddress})

	request := append(commandToBytes(COMMAND_VERSION), payload...)

	sendData(toAddress, request)
}
```

當一個節點收到Version信息，會比較自己的最大區塊高度和請求者的最大區塊高度。如果自身高度大於請求節點會向請求節點回復一個版本信息告訴請求節點自己的相關信息；否則直接向請求節點發送一個GetBlocks信息。

```
// Version命令處理器
func handleVersion(request []byte, fuu *Blockchain)  {

	var buff bytes.Buffer
	var payload Version

	dataBytes := request[COMMANDLENGTH:]

	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {

		log.Panic(err)
	}

	// 提取最大區塊高度作比較
	bestHeight := fuu.GetBestHeight()
	foreignerBestHeight := payload.BestHeight

	if bestHeight > foreignerBestHeight {

		// 向請求節點回復自身Version信息
		sendVersion(payload.AddrFrom, fuu)
	} else if bestHeight < foreignerBestHeight {

		// 向請求節點要信息
		sendGetBlocks(payload.AddrFrom)
	}

// 添加到已知節點中
	if !nodeIsKnown(payload.AddrFrom) {

		knowedNodes = append(knowedNodes, payload.AddrFrom)
	}
}
```

Blockchain獲取自身最大區塊高度的方法：

```
// 獲取區塊鏈最大高度
func (fuu *Blockchain) GetBestHeight() int64 {

	block := fuu.Iterator().Next()

	return block.Height
}
```

## GetBlocks

當一個節點知道對方節點區塊鏈最新，就需要發送一個GetBlocks請求來請求對方節點所有的區塊哈希。這裏有人覺得為什麽不直接返回對方節點所有新區塊呢，可是萬一兩個節點區塊數據相差很大，在一次請求中發送相當大的數據肯定會使通訊出問題。

```
// 表示向節點請求一個塊哈希的表，該請求會返回所有塊的哈希
type GetBlocks struct {
	//請求節點地址
	AddrFrom string
}
```

組裝發送GetBlocks消息

```
//發送COMMAND_GETBLOCKS
func sendGetBlocks(toAddress string)  {

	payload := gobEncode(GetBlocks{nodeAddress})

	request := append(commandToBytes(COMMAND_GETBLOCKS), payload...)

	sendData(toAddress, request)
}
```
當一個節點收到一個GetBlocks消息，會將自身區塊鏈所有區塊哈希算出並組裝在Inv消息中發送給請求節點。一般收到GetBlocks消息的節點為較新區塊鏈。

```
func handleGetblocks(request []byte, fuu *Blockchain)  {

	var buff bytes.Buffer
	var payload GetBlocks

	dataBytes := request[COMMANDLENGTH:]

	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blocks := fuu.GetBlockHashes()

	sendInv(payload.AddrFrom, BLOCK_TYPE, blocks)
}
```

Blockchain獲得所有區塊哈希的方法：

```
// 獲取區塊所有哈希
func (fuu *Blockchain) GetBlockHashes() [][]byte {

	blockIterator := fuu.Iterator()

	var blockHashs [][]byte

	for {

		block := blockIterator.Next()
		blockHashs = append(blockHashs, block.Hash)

		var hashInt big.Int
		hashInt.SetBytes(block.PrevBlockHash)
		if hashInt.Cmp(big.NewInt(0)) == 0 {

			break
		}
	}

	return blockHashs
}
```

## Inv消息

Inv消息用於收到GetBlocks消息的節點向其他節點展示自己擁有的區塊或交易信息。其主要結構包括自己的節點地址，展示信息的類型，是區塊還是交易，當用於節點請求區塊同步時是區塊信息；當用於節點向礦工節點轉發交易時是交易信息。

```
// 向其他節點展示自己擁有的區塊和交易
type Inv struct {
	// 自己的地址
	AddrFrom string
	// 類型 block tx
	Type     string
	// hash二維數組
	Items    [][]byte
}
```

組裝發送Inv消息：

```
//COMMAND_Inv
func sendInv(toAddress string, kind string, hashes [][]byte) {

	payload := gobEncode(Inv{nodeAddress,kind,hashes})

	request := append(commandToBytes(COMMAND_INV), payload...)

	sendData(toAddress, request)
}
```

當一個節點收到Inv消息後，會對Inv消息的類型做判斷分別采取處理。
如果是Block類型，它會取出最新的區塊哈希並組裝到一個GetData消息返回給來源節點，這個消息才是真正向來源節點請求新區塊的消息。

由於這裏將源節點(比當前節點擁有更新區塊鏈的節點)所有區塊的哈希都知道了，所以需要每處理一次Inv消息後將剩余的區塊哈希緩存到unslovedHashes數組，當unslovedHashes長度為零表示處理完畢。

這裏可能有人會有疑問，我們更新的應該是源節點擁有的新區塊(自身節點沒有)，這裏為啥請求的是全部呢？這裏的邏輯是這樣的，請求的時候是請求的全部，後面在真正更新自身數據庫的時候判斷是否為新區塊並保存到數據庫。其實，我們都知道兩個節點的區塊最大高度，這裏也可以完全請求源節點的所有新區塊哈希。為了簡單，這裏先暫且這樣處理。

如果收到的Inv是交易類型，取出交易哈希，如果該交易不存在於交易緩沖池，添加到交易緩沖池。這裏的交易類型Inv一般用於有礦工節點參與的通訊。因為在網絡中，只有礦工節點才需要去處理交易。

```
func handleInv(request []byte, fuu *Blockchain)  {

	var buff bytes.Buffer
	var payload Inv

	dataBytes := request[COMMANDLENGTH:]

	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	// Ivn 3000 block hashes [][]
	if payload.Type == BLOCK_TYPE {

		fmt.Println(payload.Items)

		blockHash := payload.Items[0]
		sendGetData(payload.AddrFrom, BLOCK_TYPE , blockHash)

		if len(payload.Items) >= 1 {

			unslovedHashes = payload.Items[1:]
		}
	}

	if payload.Type == TX_TYPE {

		txHash := payload.Items[0]

		// 添加到交易池
		if mempool[hex.EncodeToString(txHash)].TxHAsh == nil {

			sendGetData(payload.AddrFrom, TX_TYPE, txHash)
		}
	}
}
```

## GetData消息

GetData消息是用於真正請求一個區塊或交易的消息類型，其主要結構為：

```
// 用於請求區塊或交易
type GetData struct {
	// 節點地址
	AddrFrom string
	// 請求類型  是block還是tx
	Type     string
	// 區塊哈希或交易哈希
	Hash       []byte
}
```

組裝並發送GetData消息。

```
func sendGetData(toAddress string, kind string ,blockHash []byte) {

	payload := gobEncode(GetData{nodeAddress,kind,blockHash})

	request := append(commandToBytes(COMMAND_GETDATA), payload...)

	sendData(toAddress, request)
}
```

當一個節點收到GetData消息，如果是請求區塊，節點會根據區塊哈希取出對應的區塊封裝到BlockData消息中發送給請求節點；如果是請求交易，同理會根據交易哈希取出對應交易封裝到TxData消息中發送給請求節點。

```
func handleGetData(request []byte, fuu *Blockchain)  {

	var buff bytes.Buffer
	var payload GetData

	dataBytes := request[COMMANDLENGTH:]

	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {

		log.Panic(err)
	}

	if payload.Type == BLOCK_TYPE {

		block, err := fuu.GetBlock([]byte(payload.Hash))
		if err != nil {

			return
		}

		sendBlock(payload.AddrFrom, block)
	}

	if payload.Type == TX_TYPE {

		// 取出交易
		txHash := hex.EncodeToString(payload.Hash)
		tx := mempool[txHash]

		sendTx(payload.AddrFrom, &tx)
	}
}
```

Blockchain的GetBlock方法：

```
// 獲取對應哈希的區塊
func (fuu *Blockchain) GetBlock(bHash []byte) ([]byte, error)  {

	//fuuIterator := fuu.Iterator()
	//var block *Block = nil
	//var err error = nil
	//
	//for {
	//
	//	block = fuuIterator.Next()
	//	if bytes.Compare(block.Hash, bHash) == 0 {
	//
	//		break
	//	}
	//}
	//
	//if block == nil {
	//
	//	err = errors.New("Block is not found")
	//}
	//
	//return block, err

	var blockBytes []byte

	err := fuu.DB.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(blockTableName))

		if b != nil {

			blockBytes = b.Get(bHash)
		}

		return nil
	})

	return blockBytes, err
}
```

## BlockData
BlockData消息用於一個節點向其他節點發送一個區塊，到這裏才真正完成區塊的發送。

```
// 用於節點間發送一個區塊
type BlockData struct {
	// 節點地址
	AddrFrom string
	// 序列化區塊
	BlockBytes []byte
}
```
BlockData的發送：
```
func sendBlock(toAddress string, blockBytes []byte)  {


	payload := gobEncode(BlockData{nodeAddress,blockBytes})

	request := append(commandToBytes(COMMAND_BLOCK), payload...)

	sendData(toAddress, request)
}
```
當一個節點收到一個Block信息，它會首先判斷是否擁有該Block，如果數據庫沒有就將其添加到數據庫中(AddBlock方法)。然後會判斷unslovedHashes(之前緩存所有主節點未發送的區塊哈希數組)數組的長度，如果數組長度不為零表示還有未發送處理的區塊，節點繼續發送GetData消息去請求下一個區塊。否則，區塊同步完成，重置UTXO數據庫。

```
func handleBlock(request []byte, fuu *Blockchain)  {

	//fmt.Println("handleblock:\n")
	//fuu.Printchain()

	var buff bytes.Buffer
	var payload BlockData

	dataBytes := request[COMMANDLENGTH:]

	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {

		log.Panic(err)
	}

	block := DeSerializeBlock(payload.BlockBytes)
	if block == nil {

		fmt.Printf("Block nil")
	}

	err = fuu.AddBlock(block)
	if err != nil {

		log.Panic(err)
	}
	fmt.Printf("add block %x succ.\n", block.Hash)
	//fuu.Printchain()

	if len(unslovedHashes) > 0 {

		sendGetData(payload.AddrFrom, BLOCK_TYPE, unslovedHashes[0])
		unslovedHashes = unslovedHashes[1:]
	}else {

		//fuu.Printchain()

		utxoSet := &UTXOSet{fuu}
		utxoSet.ResetUTXOSet()
	}
}
```

## TxData消息

TxData消息用於真正地發送一筆交易。當對方節點發送的GetData消息為Tx類型，相應地會回復TxData消息。

```
// 同步中傳遞的交易類型
type TxData struct {
	// 節點地址
	AddFrom string
	// 交易
	TransactionBytes []byte
}
```

TxData消息的發送：

```
func sendTx(toAddress string, tx *Transaction)  {

	data := TxData{nodeAddress, tx.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes(COMMAND_TX), payload...)

	sendData(toAddress, request)
}
```

當一個節點收到TxData消息，這個節點一般為礦工節點，如果不是他會以Inv消息格式繼續轉發該交易信息到礦工節點。礦工節點收到交易，當交易池滿足一定數量時開始打包挖礦。

當生成新的區塊並打包到區塊鏈上時，礦工節點需要以BlockData消息向其他節點轉發該新區塊。

```
func handleTx(request []byte, fuu *Blockchain)  {

	var buff bytes.Buffer
	var payload TxData

	dataBytes := request[COMMANDLENGTH:]
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {

		log.Panic(err)
	}

	tx := DeserializeTransaction(payload.TransactionBytes)
	memTxPool[hex.EncodeToString(tx.TxHAsh)] = tx

	// 自身為主節點，需要將交易轉發給礦工節點
	if nodeAddress == knowedNodes[0] {

		for _, node := range knowedNodes {

			if node != nodeAddress && node != payload.AddFrom {

				sendInv(node, TX_TYPE, [][]byte{tx.TxHAsh})
			}
		}
	} else {

		//fmt.Println(len(memTxPool), len(miningAddress))
		if len(memTxPool) >= minMinerTxCount && len(miningAddress) > 0 {

		MineTransactions:

			var txs []*Transaction
			// 創幣交易，作為挖礦獎勵
			coinbaseTx := NewCoinbaseTransaction(miningAddress)
			txs = append(txs, coinbaseTx)

			var _txs []*Transaction

			for id := range memTxPool {

				tx := memTxPool[id]
				_txs = append(_txs, &tx)
				//fmt.Println("before")
				//tx.PrintTx()
				if fuu.VerifyTransaction(&tx, _txs) {

					txs = append(txs, &tx)
				}
			}

			if len(txs) == 1 {

				fmt.Println("All transactions invalid!\n")

			}

			fmt.Println("All transactions verified succ!\n")


			// 建立新區塊
			var block *Block
			// 取出上一個區塊
			err = fuu.DB.View(func(tx *bolt.Tx) error {

				b := tx.Bucket([]byte(blockTableName))
				if b != nil {

					hash := b.Get([]byte(newestBlockKey))
					block = DeSerializeBlock(b.Get(hash))
				}

				return nil
			})
			if err != nil {

				log.Panic(err)
			}

			//構造新區塊
			block = NewBlock(txs, block.Height+1, block.Hash)

			fmt.Println("New block is mined!")

			// 添加到數據庫
			err = fuu.DB.Update(func(tx *bolt.Tx) error {

				b := tx.Bucket([]byte(blockTableName))
				if b != nil {

					b.Put(block.Hash, block.Serialize())
					b.Put([]byte(newestBlockKey), block.Hash)
					fuu.Tip = block.Hash

				}
				return nil
			})
			if err != nil {

				log.Panic(err)
			}

			utxoSet := UTXOSet{fuu}
			utxoSet.Update()

			// 去除內存池中打包到區塊的交易
			for _, tx := range txs {

				fmt.Println("delete...")
				txHash := hex.EncodeToString(tx.TxHAsh)
				delete(memTxPool, txHash)
			}

			// 發送區塊給其他節點
			sendBlock(knowedNodes[0], block.Serialize())
			//for _, node := range knownNodes {
			//	if node != nodeAddress {
			//		sendInv(node, "block", [][]byte{newBlock.Hash})
			//	}
			//}

			if len(memTxPool) > 0 {

				goto MineTransactions
			}
		}
	}
}
```

好累啊，終於將一次網絡同步需要通訊的消息類型寫完了。是不是覺得好復雜，其實不然，一會結合實際🌰看過程就好理解多了。

## Server服務器端

由於我們是在本地模擬網絡環境，所以采用不同的端口號來模擬節點IP地址。eg：localhost:8000代表一個節點，eg：localhost:8001代表一個不同的節點。

寫一個啟動Server服務的方法：

```

func StartServer(nodeID string, minerAdd string) {

	// 當前節點IP地址
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	// 挖礦節點設置
	if len(minerAdd) > 0 {

		miningAddress = minerAdd
	}

	// 啟動網絡監聽服務
	ln, err := net.Listen(PROTOCOL, nodeAddress)
	if err != nil {

		log.Panic(err)
	}
	defer ln.Close()

	fuu := GetBlockchain(nodeID)
	//fmt.Println("startserver\n")
	//fuu.Printchain()

	// 第一個終端：端口為3000,啟動的就是主節點
	// 第二個終端：端口為3001，錢包節點
	// 第三個終端：端口號為3002，礦工節點
	if nodeAddress != knowedNodes[0] {

		// 該節點不是主節點，錢包節點向主節點請求數據
		sendVersion(knowedNodes[0], fuu)
	}

	for {

		// 接收客戶端發來的數據
		connc, err := ln.Accept()
		if err != nil {

			log.Panic(err)
		}

		// 不同的命令采取不同的處理方式
		go handleConnection(connc, fuu)
	}
}
```

針對不同的命令要采取不同的處理方式(上面已經講了具體命令對應的實現)，所以需要實現一個命令解析器：

```
// 客戶端命令處理器
func handleConnection(conn net.Conn, fuu *Blockchain) {

	//fmt.Println("handleConnection:\n")
	//fuu.Printchain()

	// 讀取客戶端發送過來的所有的數據
	request, err := ioutil.ReadAll(conn)
	if err != nil {

		log.Panic(err)
	}

	fmt.Printf("Receive a Message:%s\n", request[:COMMANDLENGTH])

	command := bytesToCommand(request[:COMMANDLENGTH])

	switch command {

	case COMMAND_VERSION:
		handleVersion(request, fuu)

	case COMMAND_ADDR:
		handleAddr(request, fuu)

	case COMMAND_BLOCK:
		handleBlock(request, fuu)

	case COMMAND_GETBLOCKS:
		handleGetblocks(request, fuu)

	case COMMAND_GETDATA:
		handleGetData(request, fuu)

	case COMMAND_INV:
		handleInv(request, fuu)

	case COMMAND_TX:
		handleTx(request, fuu)

	default:
		fmt.Println("Unknown command!")
	}
	defer conn.Close()
}
```

Server需要的一些全局變量：

```
//localhost:3000 主節點的地址
var knowedNodes = []string{"localhost:8000"}
var nodeAddress string //全局變量，節點地址
// 存儲擁有最新鏈的未處理的區塊hash值
var unslovedHashes [][]byte
// 交易內存池
var memTxPool = make(map[string]Transaction)
// 礦工地址
var miningAddress string
// 挖礦需要滿足的最小交易數
const minMinerTxCount = 1
```

為了能使礦工節點執行挖礦的責任，修改啟動服務的CLI代碼。當帶miner參數且不為空時，該參數為礦工獎勵地址。

```
startNodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)
flagMiner := startNodeCmd.String("miner","","定義挖礦獎勵的地址......")

```
```
func (cli *CLI) startNode(nodeID string, minerAdd string)  {

	fmt.Printf("start Server:localhost:%s\n", nodeID)
	// 挖礦地址判斷
	if len(minerAdd) > 0 {

		if IsValidForAddress([]byte(minerAdd)) {

			fmt.Printf("Miner:%s is ready to mining...\n", minerAdd)
		}else {

			fmt.Println("Server address invalid....\n")
			os.Exit(0)
		}
	}

	// 啟動服務器
	StartServer(nodeID, minerAdd)
}
```

除此之外，轉賬的send命令也需要稍作修改。帶有mine參數表示立即挖礦，由交易的第一個轉賬方地址進行挖礦；如果沒有該參數，表示由啟動服務的礦工進行挖礦。

```
flagSendBlockMine := sendBlockCmd.Bool("mine",false,"是否在當前節點中立即驗證....")
```
```
//轉賬
func (cli *CLI) send(from []string, to []string, amount []string, nodeID string, mineNow bool)  {

	fuu := GetBlockchain(nodeID)
	defer fuu.DB.Close()

	utxoSet := &UTXOSet{fuu}

	// 由交易的第一個轉賬地址進行打包交易並挖礦
	if mineNow {

		fuu.MineNewBlock(from, to, amount, nodeID)

		// 轉賬成功以後，需要更新UTXOSet
		utxoSet.Update()
	}else {
		
		// 把交易發送到礦工節點去進行驗證
		fmt.Println("miner deal with the Tx...")

		// 遍歷每一筆轉賬構造交易
		var txs []*Transaction
		for index, address := range from {

			value, _ := strconv.Atoi(amount[index])
			tx := NewTransaction(address, to[index], int64(value), utxoSet, txs, nodeID)
			txs = append(txs, tx)

			// 將交易發送給主節點
			sendTx(knowedNodes[0], tx)
		}
	}
}
```

# 網絡同步🌰詳解

假設現在的情況是這樣的：

- A節點(中心節點)，擁有3個區塊的區塊鏈
- B節點(錢包節點)，擁有1個區塊的區塊鏈
- C節點(挖礦節點)，擁有1個區塊的區塊鏈

很明顯，B節點需要向A節點請求2個區塊更新到自己的區塊鏈上。那麽，實際的代碼邏輯是怎樣處理的？

### 中心節點與錢包節點的同步邏輯
A和B都是既可以充當服務端，也可以充當客戶端。

> 1. A.StartServer 等待接收其他節點發來的消息

> 2. B.StartServer 啟動同步服務

> 3. B != 中心節點，向中心節點發請求:B.sendVersion(A, B.fuu)

> 4. A.Handle(B.Versin) :A收到B的Version消息
  > 4.1 A.fuu.Height > B.fuu.Height(3>1)  A.sendVersion(B, A.fuu)

> 5. B.Handle(A.Version):B收到A的Version消息
  5.1 B.fuu.Height > A.fuu.Height(1<3) B向A請求其所有的區塊哈希:B.sendGetBlocks(B)

> 6. A.Handle(B.GetBlocks) A將其所有的區塊哈希返回給B:A.sendInv(B, "block",blockHashes)

> 7. B.Handle(A.Inv) B收到A的Inv消息
  7.1取第一個哈希，向A發送一個消息請求該哈希對應的區塊:B.sendGetData(A, blockHash)
  7.2在收到的blockHashes去掉請求的blockHash後，緩存到一個數組unslovedHashes中

> 8. A.Handle(B.GetData) A收到B的GetData請求，發現是在請求一個區塊
  8.1 A取出對應得區塊並發送給B:A.sendBlock(B, block)

> 9. B.Handle(A.Block) B收到A的一個Block
  9.1 B判斷該Block自己是否擁有，如果沒有加入自己的區塊鏈
  9.2 len(unslovedHashes) != 0，如果還有區塊未處理，繼續發送GetData消息，相當於回7.1:B.sendGetData(A,unslovedHashes[0])
9.3 len(unslovedHashes) == 0,所有A的區塊處理完畢，重置UTXO數據庫

>10. 大功告成

### 挖礦節點參與的同步邏輯

上面的同步並沒有礦工挖礦的工作，那麽由礦工節點參與挖礦時的同步邏輯又是怎樣的呢？

> 1. A.StartServer 等待接收其他節點發來的消息

> 2. C.StartServer 啟動同步服務，並指定自己為挖礦節點，指定挖礦獎勵接收地址

> 3. C != 中心節點，向中心節點發請求:C.sendVersion(A, C.fuu)

> 4. A.Handle(C.Version),該步驟如果有更新同上面的分析相同

> 5. B.Send(B, C, amount) B給C的地址轉賬形成一筆交易
    5.1 B.sendTx(A, tx) B節點將該交易tx轉發給主節點做處理
    5.2 A.Handle(B.tx) A節點將其信息分裝到Inv發送給其他節點:A.SendInv(others, txInv)

> 6. C.Handle(A.txInv),C收到轉發的交易將其放到交易緩沖池memTxPool，當memTxPool內Tx達到一定數量就進行打包挖礦產生新區塊並發送給其他節點：C.sendBlock(others, blockData)

> 7. A(B).HandleBlock(C. blockData) A和B都會收到C產生的新區塊並添加到自己的區塊鏈上

> 8.大功告成

