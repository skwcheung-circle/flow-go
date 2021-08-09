package dns_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResolver(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(metrics.NewNoopCollector(), dns.WithBasicResolver(&basicResolver))
	require.NoError(t, err)

	txtTestCases := txtLookupFixture(10)
	ipTestCase := ipLookupFixture(10)
	wg := &sync.WaitGroup{}
	wg.Add(20) // 10 ip + 10 txt

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		go func(tc *txtLookupTestCase) {
			addrs, err := resolver.LookupTXT(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(txtTestCases[i])

		go func(tc *ipLookupTestCase) {
			addrs, err := resolver.LookupTXT(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(ipTestCase[i])
	}

	unittest.RequireReturnsBefore(t, wg.Done, 1*time.Second, "could not resolve all addresses")
}

type ipLookupTestCase struct {
	domain string
	result []net.IPAddr
}

type txtLookupTestCase struct {
	domain string
	result []string
}

// mockBasicResolverForDomains mocks the resolver for the ip and txt lookup test cases.
func mockBasicResolverForDomains(resolver *mocknetwork.BasicResolver, ipLookupTestCases []*ipLookupTestCase, txtLookupTestCases []*txtLookupTestCase) {
	for _, tc := range ipLookupTestCases {
		resolver.On("LookupIPAddr", tc.domain).Return(tc.result, nil).Once()
	}

	for _, tc := range txtLookupTestCases {
		resolver.On("LookupTXT", tc.domain).Return(tc.result, nil).Once()
	}
}

func ipLookupFixture(count int) []*ipLookupTestCase {
	tt := make([]*ipLookupTestCase, 0, count)
	for i := 0; i < count; i++ {
		tt = append(tt, &ipLookupTestCase{
			domain: fmt.Sprintf("example%d.com", i),
			result: []net.IPAddr{ // resolves each domain to 4 addresses.
				netIPAddrFixture(),
				netIPAddrFixture(),
				netIPAddrFixture(),
				netIPAddrFixture(),
			},
		})
	}

	return tt
}

func txtLookupFixture(count int) []*txtLookupTestCase {
	tt := make([]*txtLookupTestCase, 0, count)

	for i := 0; i < count; i++ {
		tt = append(tt, &txtLookupTestCase{
			domain: fmt.Sprintf("_dnsaddr.example%d.com", i),
			result: []string{ // resolves each domain to 4 addresses.
				txtIPFixture(),
				txtIPFixture(),
				txtIPFixture(),
				txtIPFixture(),
			},
		})
	}

	return tt
}

func netIPAddrFixture() net.IPAddr {
	token := make([]byte, 4)
	rand.Read(token)

	ip := net.IPAddr{
		IP:   net.IPv4(token[0], token[1], token[2], token[3]),
		Zone: "flow0",
	}

	return ip
}

func txtIPFixture() string {
	token := make([]byte, 4)
	rand.Read(token)
	return "dnsaddr=" + net.IPv4(token[0], token[1], token[2], token[3]).String()
}
