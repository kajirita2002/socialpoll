package main

import (
	"log"

	"gopkg.in/mgo.v2"
)

var db *mgo.Session

func dialdb() error {
	var err error
	log.Println("MongoDBにダイヤル中: localhost")
	db, err = mgo.Dial("localhost")
	return err
}

func closedb() {
	db.Close()
	log.Println("データベース接続が閉じられました")
}

type poll struct {
	Options []string
}

// dbから選択肢を文字列スライスとして出力する
func loadOptions() ([]string, error) {
	var options []string
	// Findによるフィルタリングを行わない
	iter := db.DB("ballots").C("polls").Find(nil).Iter()
	var p poll
	// poolオブジェクトは一つのためAllメソッドに比べてDBのメモリ使用量が少なくなる
	for iter.Next(&p) {
		options = append(options, p.Options...)
	}
	// メモリ解放
	iter.Close()
	return options, iter.Err()
}

func main() {
}