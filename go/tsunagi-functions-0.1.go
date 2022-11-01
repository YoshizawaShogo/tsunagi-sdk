package main

import (
	"sort"
	"math"
	"fmt"
	"strings"
	"net/http"
    "io/ioutil"
	"encoding/json"
	"encoding/hex"
	"golang.org/x/exp/slices"
    "golang.org/x/text/cases"
    "golang.org/x/text/language"
	"encoding/binary"
	"crypto/ed25519"
	"encoding/base32"
)
import 	"golang.org/x/crypto/sha3"

func loadCatjson( tx map[string]any, network map[string]any) []any{

	var jsonFile string
	if(tx["type"] == "AGGREGATE_COMPLETE" || tx["type"] == "AGGREGATE_BONDED"){
		jsonFile =  "aggregate.json"
	}else{
		jsonFile =  strings.ToLower(tx["type"].(string)) + ".json"
	}

	req, _ := http.NewRequest(http.MethodGet, "https://xembook.github.io/tsunagi-sdk/catjson/" + jsonFile, nil)
	resp, _ := http.DefaultClient.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)

	var result []interface{}
	json.Unmarshal([]byte(body), &result)

//	fmt.Println(result)
	return result
}

func loadLayout(tx map[string]any,catjson  []any,isEmbedded bool) []any{
	
	var prefix string
	if(isEmbedded){
		prefix = "Embedded"
	}else{
		prefix = ""
	}


	var layoutName string
	if(      tx["type"].(string) == "AGGREGATE_COMPLETE"){ layoutName = "AggregateCompleteTransaction"
	}else if(tx["type"].(string) == "AGGREGATE_BONDED"){   layoutName = "AggregateBondedTransaction"
	}else{
		layoutName = prefix + toCamelCase(tx["type"].(string)) + "Transaction"
	}
	
	idx := slices.IndexFunc(catjson, func(item any) bool {return item.(map[string]any)["factory_type"] == prefix + "Transaction" && item.(map[string]any)["name"] == layoutName})
//	return catjson[idx].(map[string]any)
	return catjson[idx].(map[string]any)["layout"].([]any)
}

func toCamelCase(str string) string {
	res := strings.Replace(cases.Title(language.Und).String(strings.Replace(str, "_", " ", -1))," ","",-1)
	return res;
}



func prepareTransaction(tx map[string]any,layout []any,network map[string]any) map[string]any{

	preparedTx := make(map[string]any)
	for k, v := range tx {
		preparedTx[k] = v
	}

	preparedTx["network"] = network["network"];
	preparedTx["version"] = network["version"];

	if _, ok := preparedTx["message"]; ok {
		message := []byte(preparedTx["message"].(string))
		preparedTx["message"] = "00" + hex.EncodeToString(message) 
	}

	if _, ok := preparedTx["name"]; ok {
		preparedTx["name"] = hex.EncodeToString([]byte(preparedTx["name"].(string))) 
	}

	if _, ok := preparedTx["value"]; ok {
		preparedTx["value"] = hex.EncodeToString([]byte(preparedTx["value"].(string))) 
	}

	if _, ok := preparedTx["mosaics"]; ok {
		castMosaics := preparedTx["mosaics"].([]any)
		sort.Slice(castMosaics, func(i, j int) bool { 
			if fmt.Sprintf("%T",castMosaics[i].(map[string]any)["mosaic_id"]) == "uint64" {
				return castMosaics[i].(map[string]any)["mosaic_id"].(uint64) < castMosaics[j].(map[string]any)["mosaic_id"].(uint64)
			}else{
				return castMosaics[i].(map[string]any)["mosaic_id"].(int) < castMosaics[j].(map[string]any)["mosaic_id"].(int)
			}
		})
	}

    for _,layer := range layout {
		layerMap := layer.(map[string]any)

		if _, ok := layerMap["size"].(string); ok{

			size := 0;

			//element_dispositionが定義されている場合は、TX内の実データをそのサイズ数で分割する。
			if _, ok := layerMap["element_disposition"]; ok{
				if _, ok := preparedTx[layerMap["name"].(string)]; ok{

					s1 := len(preparedTx[layerMap["name"].(string)].(string))
					s2 := int(layerMap["element_disposition"].(map[string]any)["size"].(float64) * 2)

					size = int( s1/s2 );
				}
			}else if strings.Contains(layerMap["size"].(string), "_count") {

				if _, ok := preparedTx[layerMap["name"].(string)]; ok{

					size = len(preparedTx[layerMap["name"].(string)].([]any));
				}else{
					size = 0;
				}
			}else{
				//その他のsize値はPayloadの長さを入れるため現時点では不明
			}
			preparedTx[layerMap["size"].(string)]  = size
		}
    }

	if _, ok := tx["transactions"]; ok{
//		txes := make([]any,len(tx["transactions"].([]any)))
		txes := make([]any,0)

		for _,eTx := range tx["transactions"].([]any){
			eTxMap := eTx.(map[string]any)
			eCatjson := loadCatjson(eTxMap,network)
			eLayout := loadLayout(eTxMap,eCatjson,true)

			//再帰処理
			ePreparedTx := prepareTransaction(eTxMap,eLayout,network)
			txes = append(txes ,ePreparedTx)
		}

		preparedTx["transactions"] = txes

	}
//	fmt.Println(preparedTx)
	return preparedTx
}

func containsKey(item map[string]any,str any) bool{

	if _, ok := item[str.(string)]; ok{

		return true
	}
	return false
}

func contains(s []int, e int) bool {
	for _, v := range s {
		if e == v {
			return true
		}
	}
	return false
}

func parseTransaction(tx  map[string]any,layout  []any,catjson  []any,network map[string]any) []any{

//	parsedTx := make([]any,len(layout))
	parsedTx := make([]any,0)

    for _,layer := range layout {
//		fmt.Println("-------------------")
		layerMap := layer.(map[string]any)
//		fmt.Println(layerMap["name"])
		layerType := layerMap["type"]
//		fmt.Println(layerType)
		layerDisposition := ""

		_=layerType
		_=layerDisposition
		if _, ok := layerMap["disposition"]; ok{
			layerDisposition = layerMap["disposition"].(string);
		}

		idx := slices.IndexFunc(catjson, func(item any) bool {
//			fmt.Println(item.(map[string]any)["name"])
			return item.(map[string]any)["name"].(string) == layerType.(string)
		})


		catitem := make(map[string]any)
//		fmt.Println(idx)
		if idx >= 0 {
			for k, v := range catjson[idx].(map[string]any) {
				catitem[k] = v
			}
		}
/*
		fmt.Println("catitem")
		
		fmt.Println(idx)
		fmt.Println(catitem)
*/
		if _, ok := layerMap["condition"]; ok{

			if layerMap["condition_operation"] == "equals" {
				if layerMap["condition_value"].(string) != tx[layerMap["condition"].(string)] {
					continue;
				}
			}
		}

		if layerDisposition == "const" {
			continue;
		}else if layerType == "EmbeddedTransaction" {

			txLayer := make(map[string]any)
			for k, v := range layerMap{
				txLayer[k] = v
			}

//			items := make([]any,len(tx["transactions"].([]any)))
			items := make([]any,0)

			for _,eTx := range tx["transactions"].([]any){
				eTxMap := make(map[string]any)
				if eTx != nil {
					eTxMap = eTx.(map[string]any)
					eCatjson := loadCatjson(eTxMap,network)
					eLayout := loadLayout(eTxMap,eCatjson,true)
	
					//再帰処理
					ePreparedTx := parseTransaction(eTxMap,eLayout,eCatjson,network)
					items = append(items ,ePreparedTx)
				}
			}

			txLayer["layout"] = items
//			fmt.Println(txLayer)
			parsedTx = append(parsedTx,txLayer)
			continue

		}else if containsKey(catitem,"layout") && containsKey(tx,layerMap["name"]) { // else:byte,struct

			txLayer := make(map[string]any)
			for k, v := range layerMap{
				txLayer[k] = v
			}

//			items := make([]any,len(tx[layerMap["name"].(string)].([]any)))
			items := make([]any,0)

			for _,item := range tx[layerMap["name"].(string)].([]any) {

				idx := slices.IndexFunc(catjson, func(item2 any) bool {return item2.(map[string]any)["name"] == layerType})
				itemParsedTx := parseTransaction(item.(map[string]any),catjson[idx].(map[string]any)["layout"].([]any),catjson,network); //再帰
				items = append(items,itemParsedTx);
			}

			txLayer["layout"] = items
//			fmt.Println(txLayer)
			parsedTx = append(parsedTx,txLayer)
			continue

		}else if layerType == "UnresolvedAddress"{
			//アドレスに30個の0が続く場合はネームスペースとみなします。
			if containsKey(tx,layerMap["name"]) && strings.Contains(tx[layerMap["name"].(string)].(string), "000000000000000000000000000000") {
				
				idx := slices.IndexFunc(catjson, func(item any) bool {return item.(map[string]any)["name"] == "NetworkType"})
				idx2 := slices.IndexFunc(catjson[idx].(map[string]any)["values"].([]any), func(item any) bool {return item.(map[string]any)["name"].(string) == tx["network"].(string)})
//				fmt.Println("idx2")
//				fmt.Println(catjson[idx].(map[string]any)["values"].([]any)[idx2].(map[string]any)["value"])
				prefix := fmt.Sprintf("%x", int(catjson[idx].(map[string]any)["values"].([]any)[idx2].(map[string]any)["value"].(float64)) + 1)
				
				tx[layerMap["name"].(string)] =  prefix + tx[layerMap["name"].(string)].(string);
			}
		}else if catitem["type"] == "enum" {

			if strings.Contains(catitem["name"].(string),"Flags") {

				value := 0
				for _,itemLayer := range catitem["values"].([]any) {
					if strings.Contains(tx[layerMap["name"].(string)].(string),itemLayer.(map[string]any)["name"].(string)) {
						value += int(itemLayer.(map[string]any)["value"].(float64))
					}
				}
				catitem["value"] = value

			}else if strings.Contains(layerDisposition,"array") {
//				values := make([]any,len(tx[layerMap["name"].(string)].([]any)))
				values := make([]any,0)
				for _,item := range tx[layerMap["name"].(string)].([]any) {
					idx := slices.IndexFunc(catitem["values"].([]any), func(value any) bool {return value.(map[string]any)["name"] == item.(string)})
					values = append(values,catitem["values"].([]any)[idx].(map[string]any)["value"])
				}
				tx[layerMap["name"].(string)] = values
			}else{
				idx := slices.IndexFunc(catitem["values"].([]any), func(item any) bool {return item.(map[string]any)["name"] == tx[layerMap["name"].(string)]})
				if idx >= 0 {
					catitem["value"] = catitem["values"].([]any)[idx].(map[string]any)["value"]
				}
			}
		}

		if strings.Contains(layerDisposition,"array") {
			if layerType == "byte" {
				size := tx[layerMap["size"].(string)].(int)
				if containsKey(layerMap,"element_disposition") {
					subLayout := make(map[string]any)
					for k, v := range layerMap {
						subLayout[k] = v
					}

					items := make([]any,0)
					for i := 0; i < size; i++ {
						txLayer := make(map[string]any)
						txLayer["signedness"] = layerMap["element_disposition"].(map[string]any)["signedness"]
						txLayer["name"] = "element_disposition"
						txLayer["size"] = layerMap["element_disposition"].(map[string]any)["size"]
						txLayer["value"] = tx[layerMap["name"].(string)].(string)[i*2:i*2+2]
						txLayer["type"] = layerType
						
						items = append(items,txLayer)
					}
					subLayout["layout"] = items
					parsedTx = append(parsedTx,subLayout)


				}else{
					fmt.Println("not implemented yet")

				}
			}else if containsKey(tx,layerMap["name"]) {
				subLayout := make(map[string]any)
				for k, v := range layerMap {
					subLayout[k] = v
				}
//				items := make([]any,len(tx[layerMap["name"].(string)].([]any)))
				items := make([]any,0)
				for _,txItem := range tx[layerMap["name"].(string)].([]any) {
					idx := slices.IndexFunc(catjson, func(item any) bool {
						return item.(map[string]any)["name"] == layerType
					})
			
					txLayer := make(map[string]any)
					if idx >= 0 {
						for k, v := range catjson[idx].(map[string]any) {
							txLayer[k] = v
						}
					}
					txLayer["value"] = txItem

					if layerType == "UnresolvedAddress"{
						if strings.Contains(txItem.(string),"000000000000000000000000000000") {

							idx := slices.IndexFunc(catjson, func(item any) bool {return item.(map[string]any)["name"] == "NetworkType"})
							idx2 := slices.IndexFunc(catjson[idx].(map[string]any)["values"].([]any), func(item any) bool {return item.(map[string]any)["name"].(string) == tx["network"].(string)})
//							fmt.Println("idx2")
//							fmt.Println(catjson[idx].(map[string]any)["values"].([]any)[idx2].(map[string]any)["value"])
							prefix := fmt.Sprintf("%x", int(catjson[idx].(map[string]any)["values"].([]any)[idx2].(map[string]any)["value"].(float64)) + 1)
										
							txLayer["value"] =  prefix + txLayer["value"].(string)
						}
					}
					items = append(items,txLayer)
				}
				subLayout["layout"] = items

//				fmt.Println(subLayout)
				parsedTx = append(parsedTx,subLayout)

			}else{
				fmt.Println("not implemented yet")

			}
		}else{

			txLayer := make(map[string]any)
			for k, v := range layerMap {
				txLayer[k] = v

			}
			if len(catitem) > 0 {
				txLayer["signedness"] = catitem["signedness"]
				txLayer["type"] = catitem["type"]
				txLayer["value"] = catitem["value"]
				txLayer["size"] = catitem["size"]
			}

			if containsKey(tx,layerMap["name"]) && catitem["type"] != "enum" {
				txLayer["value"] = tx[layerMap["name"].(string)]
			}else{
			}

//			fmt.Println(txLayer)
			parsedTx = append(parsedTx,txLayer)
		}
	}
	idx := slices.IndexFunc(parsedTx, func(item any) bool {
		if item != nil {
			return item.(map[string]any)["name"] == "size"
		}
		return false

	})
	if idx >= 0 {
		layerSize := parsedTx[idx].(map[string]any)
		layerSize["value"] = countSize(parsedTx,0)
	}

//	fmt.Println(parsedTx)

	return parsedTx
}

func countSize(item any, alignment int) int {

	totalSize := 0
//	fmt.Println(item)
	if fmt.Sprintf("%T",item) == "[]interface {}" {
		layoutSize := 0
		for _,layout := range item.([]any) {
			layoutSize += countSize(layout.(map[string]any),alignment)
		}
		if alignment > 0 {
			layoutSize = int((layoutSize + alignment - 1)/alignment) *  alignment
		}
		totalSize += layoutSize

	}else if containsKey(item.(map[string]any),"layout") {
		for _,layer := range item.(map[string]any)["layout"].([]any) {
			itemAlignment := 0
			if containsKey(item.(map[string]any),"alignment") {
				itemAlignment = int(item.(map[string]any)["alignment"].(float64))
			}else{
				itemAlignment = 0
			}
			totalSize += countSize(layer,itemAlignment)
		}
	}else{
		if containsKey(item.(map[string]any),"size") {
			totalSize += int(item.(map[string]any)["size"].(float64))
		}
	}
	return totalSize
}

func buildTransaction(parsedTx []any) []any {
	builtTx := make([]any,0)

	for idx := range parsedTx {
		builtTx = append(builtTx,parsedTx[idx])
	}		

	layerPayloadSize := make(map[string]any)
	idx := slices.IndexFunc(builtTx, func(item any) bool {return item.(map[string]any)["name"] == "payload_size"})
	if idx >= 0 {
		layerPayloadSize = builtTx[idx].(map[string]any)
		idx2 := slices.IndexFunc(builtTx, func(item any) bool {return item.(map[string]any)["name"] == "transactions"})
		if idx2 >= 0 {
			layerPayloadSize["value"] = countSize(builtTx[idx2].(map[string]any),0)
		}
	}

	layerTransactionsHash := make(map[string]any)
	idx = slices.IndexFunc(builtTx, func(item any) bool {return item.(map[string]any)["name"] == "transactions_hash"})
	if idx >= 0 {
		layerTransactionsHash = builtTx[idx].(map[string]any)
		hashes := make([]any,0)

		idx2 := slices.IndexFunc(builtTx, func(item any) bool {return item.(map[string]any)["name"] == "transactions"})
		if idx2 >= 0 {
			txLayout := builtTx[idx2].(map[string]any)["layout"].([]any)
			for _,eTx := range txLayout {
				hexedString,_ := hex.DecodeString(hexlifyTransaction(eTx,0))
				hashes = append(hashes,sha3.Sum256([]byte(hexedString)))
			}
		}
		 
		numRemainingHashes := len(hashes)
		for numRemainingHashes > 1 {
			i := 0
			for i < numRemainingHashes {
				hasher := sha3.New256()
//				fmt.Printf("%T",hashes[i])
//				fmt.Println(hashes[i])
				//fmt.Sprintf("%T",item)

				if(fmt.Sprintf("%T",hashes[i]) == "[]uint8") {
					hasher.Write(hashes[i].([]byte))
				}else{
					arrayHashi := hashes[i].([32]byte)
					byteHashi := []byte(arrayHashi[0:len(arrayHashi)])
					hasher.Write(byteHashi)
				}
		
//				hasher.Write(byteHashi)
				
				if i+1 < numRemainingHashes {

					if(fmt.Sprintf("%T",hashes[i]) == "[]uint8") {
						hasher.Write(hashes[i+1].([]byte))
					}else{
						arrayHaship1 := hashes[i+1].([32]byte)
						byteHaship1 := []byte(arrayHaship1[0:len(arrayHaship1)])
						hasher.Write(byteHaship1)
					}					



				}else{
					if(fmt.Sprintf("%T",hashes[i]) == "[]uint8") {
						hasher.Write(hashes[i].([]byte))
					}else{
						arrayHashi := hashes[i].([32]byte)
						byteHashi := []byte(arrayHashi[0:len(arrayHashi)])
						hasher.Write(byteHashi)
					}					
					//hasher.Write(byteHashi)
					numRemainingHashes += 1
				}
				hashes[i/2] = hasher.Sum(nil)
				i += 2
			}
			numRemainingHashes = int(numRemainingHashes/2)
		}

		if(fmt.Sprintf("%T",hashes[0]) == "[]uint8") {
			layerTransactionsHash["value"] = hex.EncodeToString(hashes[0].([]byte))
		}else{
			arrayHash0 := hashes[0].([32]byte)
			byteHash0 := []byte(arrayHash0[0:len(arrayHash0)])
			layerTransactionsHash["value"] = hex.EncodeToString(byteHash0)
		}	
//		arrayHash0 := hashes[0].([32]byte)
//		byteHash0 := []byte(arrayHash0[0:len(arrayHash0)])
//		layerTransactionsHash["value"] = hex.EncodeToString(byteHash0)
	}
//	fmt.Println(layerTransactionsHash["value"])
	return builtTx
}

func hexlifyTransaction(item any, alignment int)string{
	
	payload := ""
	if fmt.Sprintf("%T",item) == "[]interface {}" {
		subLayoutHex := ""
		for _,layout := range item.([]any) {
//			fmt.Println("layout")
//			fmt.Println(layout.(map[string]any)["name"])
			subLayoutHex += hexlifyTransaction(layout.(map[string]any),alignment)
//			fmt.Println(subLayoutHex)
		}
		if alignment > 0 {
			alignedSize := math.Floor(float64(len(subLayoutHex) + (alignment * 2) - 2)/float64(alignment * 2)) * float64(alignment * 2)
			//alignedSize := math.Floor(  (len(subLayoutHex) + (alignment * 2) - 2)/(alignment * 2)) * float64(alignment * 2)
			subLayoutHex = subLayoutHex + strings.Repeat("0",int(alignedSize) - len(subLayoutHex))
		}	
		payload += subLayoutHex
	}else if containsKey(item.(map[string]any),"layout") {
		for _,layer := range item.(map[string]any)["layout"].([]any) {
			itemAlignment := 0
			if containsKey(item.(map[string]any),"alignment") {
				itemAlignment = int(item.(map[string]any)["alignment"].(float64))
			}else{
				itemAlignment = 0
			}
			payload += hexlifyTransaction(layer,itemAlignment)
		}
	}else{
//		fmt.Println("else")
		size := int(item.(map[string]any)["size"].(float64))
		
		if !containsKey(item.(map[string]any),"value") {
			if size >= 24 {
				item.(map[string]any)["value"] = strings.Repeat("00",size)
			}else{
				item.(map[string]any)["value"] = 0
			}
		}
		if size == 1 {
			if item.(map[string]any)["name"] == "element_disposition" {
				payload = item.(map[string]any)["value"].(string)
			}else{
				varint := make([]byte, 2)
				if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "int" {
					binary.PutUvarint(varint,uint64(item.(map[string]any)["value"].(int)))
				}else if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "float64" {
					binary.PutUvarint(varint,uint64(item.(map[string]any)["value"].(float64)))
				}
				payload = hex.EncodeToString(varint[:1])
			}
		}else if size == 2 {
			varint := make([]byte, 2)
			if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "int" {
				binary.LittleEndian.PutUint16(varint,uint16(item.(map[string]any)["value"].(int)))
			}else if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "float64" {
				binary.LittleEndian.PutUint16(varint,uint16(item.(map[string]any)["value"].(float64)))
			}
			payload = hex.EncodeToString(varint)
//			fmt.Println(item.(map[string]any)["name"])
//			fmt.Println(payload)

		}else if size == 4 {
		
			varint := make([]byte, 4)
			if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "int" {
				binary.LittleEndian.PutUint32(varint,uint32(item.(map[string]any)["value"].(int)))
			}else if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "float64" {
				binary.LittleEndian.PutUint32(varint,uint32(item.(map[string]any)["value"].(float64)))
			}
			payload = hex.EncodeToString(varint)

		}else if size == 8 {
			varint := make([]byte, 8)
			if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "int" {
				binary.LittleEndian.PutUint64(varint,uint64(item.(map[string]any)["value"].(int)))
			}else if fmt.Sprintf("%T",item.(map[string]any)["value"]) == "uint64" {
				binary.LittleEndian.PutUint64(varint,uint64(item.(map[string]any)["value"].(uint64)))
			}
			payload = hex.EncodeToString(varint)
//			fmt.Println("-----------------------")
//			fmt.Println(item.(map[string]any)["name"])
//			fmt.Println(payload)
		
		}else if size == 24 || size == 32 || size == 64 {
			payload = item.(map[string]any)["value"].(string)
		}else{
			fmt.Println("Unknown size")
			fmt.Println(size)

		}
	}
	return payload
}

func signTransaction(builtTx []any, priKey string,network map[string]any) string{
	seed, _ := hex.DecodeString(priKey)
	privateKey := ed25519.NewKeyFromSeed(seed)
	verifiableData := getVerifiableData(builtTx)
	payload := network["generationHash"].(string) + hexlifyTransaction(verifiableData,0)

	verifiableBuffer, _ := hex.DecodeString(payload)
	signature := ed25519.Sign(privateKey, verifiableBuffer)
//	fmt.Println(hex.EncodeToString(signature))
	return hex.EncodeToString(signature)
}




func getVerifiableData(builtTx []any)[]any{
	idx := slices.IndexFunc(builtTx, func(item any) bool {return item.(map[string]any)["name"] == "type"})
	if idx >= 0 {
		typeLayer := builtTx[idx].(map[string]any)["value"]
		if(contains([]int{16705, 16961}, int(typeLayer.(float64)))){
//			fmt.Println(builtTx[5:11])
			return  builtTx[5:11]
		}else{
			return builtTx[5:]
		}
	}
	return make([]any,0)
}

func hashTransaction(signer string,signature string,builtTx []any,network map[string]any) string{

	hasher := sha3.New256()
	//decodeStr := []byte
	decodeStr,_ := hex.DecodeString(signature)
	hasher.Write(decodeStr)
	decodeStr,_ = hex.DecodeString(signer)
	hasher.Write(decodeStr)
	decodeStr,_ = hex.DecodeString(network["generationHash"].(string))
	hasher.Write(decodeStr)
	decodeStr,_ = hex.DecodeString(hexlifyTransaction(getVerifiableData(builtTx),0))
	hasher.Write(decodeStr)
	return hex.EncodeToString(hasher.Sum(nil))
}

//func updateTransaction(builtTx []any,name string,typeString string,value string) []any{
func updateTransaction(builtTx []any,name string,typeString string,value any) []any{

	updatedTx := make([]any,0)
	for idx := range builtTx {
		updatedTx = append(updatedTx,builtTx[idx])
	}	

	idx := slices.IndexFunc(updatedTx, func(item any) bool {return item.(map[string]any)["name"] == name})
	if idx >= 0 {
		updatedTx[idx].(map[string]any)[typeString] = value
	}
	return updatedTx
}

func cosignTransaction(txHash string,priKey string) string{
	seed, _ := hex.DecodeString(priKey)
	privateKey := ed25519.NewKeyFromSeed(seed)
	decodeStr,_ := hex.DecodeString(txHash)
	signature := ed25519.Sign(privateKey, decodeStr)
	return hex.EncodeToString(signature)
}

func generateAddressId(address string) string{

	recipientAddress, _ := base32.StdEncoding.DecodeString(address + "A")
	return hex.EncodeToString(recipientAddress[:len(recipientAddress) - 1])
	//return recipientAddress
}

func generateNamespaceId(name string,parentNamespaceId uint64) uint64{

	namespace_flag := uint64(1 << 63)
	hasher := sha3.New256()
	varint1 := make([]byte, 4)
	binary.LittleEndian.PutUint32(varint1,uint32(parentNamespaceId & 0xFFFFFFFF))
	hasher.Write(varint1)

	varint2 := make([]byte, 4)
	binary.LittleEndian.PutUint32(varint2,uint32((parentNamespaceId >> 32) & 0xFFFFFFFF))
	hasher.Write(varint2)

	hasher.Write([]byte(name))
	digest := hasher.Sum(nil)
//	fmt.Println(digest)
//	namespaceId := sha3.Sum256([]byte(namespace))
	digestToBigint(digest)
//	fmt.Println(digestToBigint(digest) | namespace_flag)
return digestToBigint(digest) | namespace_flag
}

func digestToBigint(digest []byte) uint64{
//	varint := make([]byte, 8)
//	binary.LittleEndian.PutUint64(varint,uint64(digest))
	result := uint64(0)
	for i := 0; i < 8; i++ {
		result |= uint64(digest[i]) << (8 * i)
	}

	return result
}

func convertAddressAliasId(namespaceId uint64) string{
	//namespaceId = namespaceId & 0xFFFFFFFFFFFFFFFF){
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, namespaceId)

	return hex.EncodeToString(b) + "000000000000000000000000000000";
}

func generateMosaicId(ownerAddress string, nonce int) uint64{

	namespace_flag := uint64(1 << 63)
	hasher := sha3.New256()

	varint1 := make([]byte, 4)
	binary.LittleEndian.PutUint32(varint1,uint32(nonce))
	hasher.Write(varint1)

	hexedString,_ := hex.DecodeString(ownerAddress)
	hasher.Write(hexedString)
	result := digestToBigint(hasher.Sum(nil))

	if result & namespace_flag > 0 {
		result -= namespace_flag
	}

	return result
}
