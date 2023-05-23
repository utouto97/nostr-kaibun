package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	feedRelays = []string{
		"wss://relay-jp.nostr.wirednet.jp",
		// "wss://nostr-relay.nokotaro.com",
		// "wss://nostr.holybea.com",
		// "wss://relay.snort.social",
		"wss://relay.damus.io",
		// "wss://nostr.h3z.jp",
	}
	postRelays = []string{
		"wss://relay-jp.nostr.wirednet.jp",
		// "wss://nostr-relay.nokotaro.com",
		// "wss://nostr.holybea.com",
		"wss://relay.snort.social",
		"wss://relay.damus.io",
		// "wss://nostr.h3z.jp",
	}
	nsec = os.Getenv("NSEC")
)

type pool struct {
	nostr.SimplePool
}

func (p *pool) Publish(ctx context.Context, urls []string, event *nostr.Event) uint64 {
	var wg sync.WaitGroup
	var cnt uint64

	for _, url := range urls {
		wg.Add(1)
		go func(nm string) {
			defer wg.Done()

			r, err := p.EnsureRelay(nm)
			if err != nil {
				return
			}

			if _, err := r.Publish(ctx, *event); err != nil {
				return
			}

			atomic.AddUint64(&cnt, 1)
		}(nostr.NormalizeURL(url))
	}

	wg.Wait()
	return cnt
}

func isKaibun(s string) bool {
	s = strings.TrimSpace(s)
	t := []rune(s)
	n := len(t)
	if n < 2 {
		return false
	}
	for i := 0; i < n/2; i++ {
		if t[i] != t[n-1-i] {
			return false
		}
	}
	return true
}

func main() {
	if nsec == "" {
		panic(errors.New("nsec is empty"))
	}

	var sk string
	if _, s, err := nip19.Decode(nsec); err == nil {
		sk = s.(string)
	} else {
		panic(err)
	}

	ctx := context.Background()
	pool := pool{*nostr.NewSimplePool(ctx)}

	ts := nostr.Timestamp(time.Now().Unix())
	filters := []nostr.Filter{{
		Kinds: []int{nostr.KindTextNote},
		Since: &ts,
	}}
	events := pool.SubMany(ctx, feedRelays, filters)
	for e := range events {
		c := strings.ToLower(strings.Join(strings.Fields(e.Content), ""))
		if isKaibun(c) {
			fmt.Println(e.PubKey, c)
			ne := nostr.Event{
				CreatedAt: nostr.Now(),
				Kind:      nostr.KindTextNote,
				Tags: []nostr.Tag{
					{"e", e.ID},
					{"p", e.PubKey},
				},
				Content: "回文!",
			}
			ne.Sign(sk)

			ctx, cancel := context.WithTimeout(ctx, time.Duration(3*time.Second))
			pool.Publish(ctx, postRelays, &ne)
			cancel()
		}
	}
}
