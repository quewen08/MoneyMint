// Command seed 是一次性维护工具：给现有数据库中的每个账本补默认账户
// （支出/收入/资产/负债/权益）。已存在的同名账户会自动跳过，可重复执行。
//
// 用法（在项目根目录）：
//
//	go run ./cmd/seed
//
// 环境变量 LEDGER_DB / LEDGER_SCHEMA 同服务端（默认 data/ledger.db、schema.sql）。
package main

import (
	"fmt"
	"os"

	"familyledger/internal/config"
	"familyledger/internal/db"
	"familyledger/internal/repository"
)

func main() {
	cfg := config.Load()
	conn, err := db.Open(cfg.DBPath, cfg.SchemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := repository.New(conn)

	var ids []int64
	rows, err := conn.Query(`SELECT id FROM ledgers ORDER BY id`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询账本失败: %v\n", err)
		os.Exit(1)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			fmt.Fprintf(os.Stderr, "读取账本失败: %v\n", err)
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) == 0 {
		fmt.Println("没有账本，无需 seed。")
		return
	}

	for _, id := range ids {
		if err := store.SeedDefaultAccounts(id); err != nil {
			fmt.Fprintf(os.Stderr, "账本 %d seed 失败: %v\n", id, err)
			os.Exit(1)
		}
		fmt.Printf("账本 %d：已写入/确认默认账户。\n", id)
	}
	fmt.Println("完成。")
}
