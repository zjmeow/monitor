package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/time/rate"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"time"
)

var keyToRMB = map[string]float64{
	"XSOL":    3.6,
	"HYUSD":   7.2,
	"JITOSOL": 1152,
	"WSOL":    1000,
	"BSOL":    1200,
}

type Monitor struct {
	solClient *rpc.Client
	address   solana.PublicKey
	interval  time.Duration
	balance   map[solana.PublicKey]float64
	mints     map[string]solana.PublicKey // 名称 -> 代币地址
}

func NewMonitor(address solana.PublicKey, interval time.Duration, mints map[string]solana.PublicKey) *Monitor {
	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(
		"https://mainnet.helius-rpc.com/?api-key=5dc806ed-38ff-4b3f-9724-217b6445ba44",
		rate.Every(time.Second),
		100,
	))
	return &Monitor{
		solClient: rpcClient,
		address:   address,
		interval:  interval,
		balance:   make(map[solana.PublicKey]float64),
		mints:     mints,
	}
}

func (m *Monitor) Run() {
	ticker := time.NewTicker(m.interval)
	// 当有变化的时候计算出差值并发送通知
	for range ticker.C {
		for name, mint := range m.mints {
			_, balance, err := m.GetAtaBalance(m.address, mint)
			if err != nil {
				continue
			}
			before := m.balance[mint]
			after := balance
			// 排除掉初始化时的差异
			if before != 0 && before != after {
				m.sendNotification(name, before, after)
			} else {
				fmt.Printf("nochange name: %s, balance: %v\n", name, after)
			}
			m.balance[mint] = after
		}
	}
}

func (m *Monitor) sendNotification(name string, before, after float64) {
	// 发送 http
	diff := after - before
	title := fmt.Sprintf("赚钱了！ %.4f RMB！%s利润: %.4f;余额%.4f->%.4f", keyToRMB[name]*diff, name, diff, before, after)
	fmt.Println(title)
	url := fmt.Sprintf("https://sctapi.ftqq.com/SCT130069TmrwzmLAkwkYSqi6grDbW2kTA.send?title=%s&desp=messagecontent", title)
	response, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return
	}
	fmt.Println(string(body))
}

func (m *Monitor) GetAtaBalance(address, mint solana.PublicKey) (uint64, float64, error) {
	ata, _, err := solana.FindAssociatedTokenAddress(address, mint)
	if err != nil {
		return 0, 0, err
	}
	balanceResp, err := m.solClient.GetTokenAccountBalance(context.TODO(), ata, rpc.CommitmentFinalized)
	if err != nil {
		return 0, 0, err
	}
	balance := balanceResp.Value.Amount
	uiAmount := balanceResp.Value.UiAmountString
	uiAmountFloat, err := strconv.ParseFloat(uiAmount, 64)
	if err != nil {
		return 0, 0, err
	}
	bigAmount, suc := new(big.Int).SetString(balance, 10)
	if !suc {
		return 0, 0, errors.New("failed to parse balance")
	}
	return bigAmount.Uint64(), uiAmountFloat, nil
}

func main() {
	m := NewMonitor(
		solana.MustPublicKeyFromBase58("4uo4N7Q6GZS4TZtpXCZVfrUhM7mnkyVUEvkrWWXoCqEv"),
		3*time.Minute,
		map[string]solana.PublicKey{
			"XSOL":    solana.MustPublicKeyFromBase58("4sWNB8zGWHkh6UnmwiEtzNxL4XrN7uK9tosbESbJFfVs"),
			"HYUSD":   solana.MustPublicKeyFromBase58("5YMkXAYccHSGnHn9nob9xEvv6Pvka9DZWH7nTbotTu9E"),
			"JITOSOL": solana.MustPublicKeyFromBase58("J1toso1uCk3RLmjorhTtrVwY9HJ7X8V9yYac6Y7kGCPn"),
			"WSOL":    solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"),
			"BSOL":    solana.MustPublicKeyFromBase58("bSo13r4TkiE4KumL71LsHTPpL2euBYLFx6h9HP3piy1"),
		},
	)
	m.Run()
}
