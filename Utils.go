/**
@author: chaors

@file:   utils.go

@time:   2018/06/21 22:06

@desc:   一些常用的辅助方法
*/


package BLC

import (
	"bytes"
	"encoding/binary"
	"log"
	"encoding/json"
	"encoding/gob"
	"fmt"
)

//将int64转换为bytes
func IntToHex(num int64) []byte  {

	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {

		log.Panic(err)
	}

	return buff.Bytes()
}

// 标准的JSON字符串转数组
func Json2Array(jsonString string) []string {

	//json 到 []string
	var sArr []string
	if err := json.Unmarshal([]byte(jsonString), &sArr); err != nil {

		log.Panic(err)
	}
	return sArr
}


// 字节数组反转
func ReverseBytes(data []byte) {

	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {

		data[i], data[j] = data[j], data[i]
	}
}


// 将结构体序列化成字节数组
func gobEncode(data interface{}) []byte {

	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func commandToBytes(command string) []byte {

	// 消息在底层就是字节序列,前12个字节指定了命令名（比如这里的 version）
	var bytes [COMMANDLENGTH]byte

	for i, c := range command {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

func bytesToCommand(bytes []byte) string {
	var command []byte

	for _, b := range bytes {
		if b != 0x0 {
			command = append(command, b)
		}
	}

	return fmt.Sprintf("%s", command)
}