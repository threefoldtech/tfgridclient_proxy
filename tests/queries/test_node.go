package main

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid_proxy_server/pkg/gridproxy"
)

var (
	statusUP = "up"
)

var (
	returned = make(map[int]uint64)
)

const (
	nodeStateFactor = 3
	reportInterval  = time.Hour
	Tests           = 2000
)

type NodesAggregate struct {
	countries []string
	cities    []string
	farmNames []string
	farmIDs   []uint64
	freeMRUs  []uint64
	freeSRUs  []uint64
	freeHRUs  []uint64

	maxFreeMRU  uint64
	maxFreeSRU  uint64
	maxFreeHRU  uint64
	maxFreeIPs  uint64
	nodeRenters []uint64
	twins       []uint64
}

func nodeSatisfies(data *DBData, node node, f gridproxy.NodeFilter) bool {
	if f.Status != nil && (*f.Status == "up") != isUp(node.updated_at) {
		return false
	}
	total := data.nodeTotalResources[node.node_id]
	used := data.nodeUsedResources[node.node_id]
	free := calcFreeResources(total, used)
	if f.FreeMRU != nil && *f.FreeMRU > free.mru {
		return false
	}
	if f.FreeHRU != nil && *f.FreeHRU > free.hru {
		return false
	}
	if f.FreeSRU != nil && *f.FreeSRU > free.sru {
		return false
	}
	if f.Country != nil && *f.Country != node.country {
		return false
	}
	if f.City != nil && *f.City != node.city {
		return false
	}
	if f.FarmName != nil && *f.FarmName != data.farms[node.farm_id].name {
		return false
	}
	if f.FarmIDs != nil && !isIn(f.FarmIDs, node.farm_id) {
		return false
	}
	if f.FreeIPs != nil && *f.FreeIPs > data.FreeIPs[node.farm_id] {
		return false
	}
	if f.IPv4 != nil && *f.IPv4 && data.publicConfigs[node.node_id].ipv4 == "" {
		return false
	}
	if f.IPv6 != nil && *f.IPv6 && data.publicConfigs[node.node_id].ipv6 == "" {
		return false
	}
	if f.Domain != nil && *f.Domain && data.publicConfigs[node.node_id].domain == "" {
		return false
	}
	rentable := data.farms[node.farm_id].dedicated_farm && data.nodeRentedBy[node.node_id] == 0
	if f.Rentable != nil && *f.Rentable != rentable {
		return false
	}
	if f.RentedBy != nil && *f.RentedBy != data.nodeRentedBy[*f.RentedBy] {
		return false
	}
	if f.AvailableFor != nil && (*f.AvailableFor != data.nodeRentedBy[*f.AvailableFor] && data.farms[node.farm_id].dedicated_farm) {
		return false
	}
	return true
}

func validateResults(local, remote []gridproxy.Node) error {
	iter := local
	if len(remote) < len(local) {
		iter = remote
	}
	for i := range iter {
		if !reflect.DeepEqual(local[i], remote[i]) {
			return fmt.Errorf("node %d mismatch: local: %+v, remote: %+v", i, local[i], remote[i])
		}
	}

	if len(local) != len(remote) {
		if len(local) == 0 {
			return fmt.Errorf("local empty but remote returned: %+v", remote[0])
		} else if len(remote) == 0 {
			return fmt.Errorf("remote empty but local returned: %+v", local[0])
		}
		return errors.New("length mismatch")
	}
	return nil
}

func calcNodesAggregates(data *DBData) (res NodesAggregate) {
	cities := make(map[string]struct{})
	countries := make(map[string]struct{})
	for _, node := range data.nodes {
		cities[node.city] = struct{}{}
		countries[node.country] = struct{}{}
		countries[data.farms[node.farm_id].name] = struct{}{}
		free := calcFreeResources(data.nodeTotalResources[node.node_id], data.nodeUsedResources[node.node_id])
		res.maxFreeHRU = max(free.hru, free.hru)
		res.maxFreeSRU = max(free.hru, free.sru)
		res.maxFreeMRU = max(free.hru, free.mru)
		res.freeMRUs = append(res.freeMRUs, free.mru)
		res.freeSRUs = append(res.freeSRUs, free.sru)
		res.freeHRUs = append(res.freeHRUs, free.hru)
	}
	for _, contract := range data.rent_contracts {
		if contract.state != "Created" {
			continue
		}
		res.nodeRenters = append(res.nodeRenters, contract.twin_id)
	}
	for _, twin := range data.twins {
		res.twins = append(res.twins, twin.twin_id)
	}
	for city := range cities {
		res.cities = append(res.cities, city)
	}
	for country := range countries {
		res.countries = append(res.cities, country)
	}
	for _, farm := range data.farms {
		res.farmNames = append(res.farmNames, farm.name)
		res.farmIDs = append(res.farmIDs, farm.farm_id)
	}

	farmIPs := make(map[uint64]uint64)
	for _, publicIP := range data.publicIPs {
		if publicIP.contract_id == 0 {
			farmIPs[data.farmIDMap[publicIP.farm_id]] += 1
		}
	}
	for _, cnt := range farmIPs {
		res.maxFreeIPs = max(res.maxFreeIPs, cnt)
	}
	return
}

func NodeUpTest(data *DBData, proxyClient, localClient gridproxy.GridProxyClient) error {
	f := gridproxy.NodeFilter{
		Status: &statusUP,
	}
	l := gridproxy.Limit{
		Size:     999999999,
		Page:     1,
		RetCount: true,
	}
	localNodes, err := localClient.Nodes(f, l)
	if err != nil {
		return err
	}
	remoteNodes, err := proxyClient.Nodes(f, l)
	if err != nil {
		return err
	}
	if err := validateResults(localNodes, remoteNodes); err != nil {
		return err
	}
	return nil
}

func randomNodeFilter(agg *NodesAggregate) gridproxy.NodeFilter {
	var f gridproxy.NodeFilter
	if flip(.5) { // status
		status := "down"
		if flip(.5) {
			status = "up"
		}
		f.Status = &status
	}
	if flip(.5) {
		if flip(.1) {
			c := agg.freeMRUs[rand.Intn(len(agg.freeMRUs))]
			f.FreeMRU = &c
		} else {
			f.FreeMRU = rndref(0, agg.maxFreeMRU)
		}
	}
	if flip(.5) {
		if flip(.1) {
			c := agg.freeHRUs[rand.Intn(len(agg.freeHRUs))]
			f.FreeHRU = &c
		} else {
			f.FreeHRU = rndref(0, agg.maxFreeHRU)
		}
	}
	if flip(.5) {
		if flip(.1) {
			c := agg.freeSRUs[rand.Intn(len(agg.freeSRUs))]
			f.FreeSRU = &c
		} else {
			f.FreeSRU = rndref(0, agg.maxFreeSRU)
		}
	}
	if flip(.05) {
		c := agg.countries[rand.Intn(len(agg.countries))]
		f.Country = &c
	}
	if flip(.05) {
		c := agg.farmNames[rand.Intn(len(agg.farmNames))]
		f.FarmName = &c
	}
	if flip(.05) {
		for _, id := range agg.farmIDs {
			if flip(float32(min(3, uint64(len(agg.farmIDs)))) / float32(len(agg.farmIDs))) {
				f.FarmIDs = append(f.FarmIDs, id)
			}
		}
	}
	if flip(.05) {
		f.FreeIPs = rndref(0, agg.maxFreeIPs)
	}
	if flip(.05) {
		v := true
		// if flip(.5) {
		// 	v = false
		// }
		f.IPv4 = &v
	}
	if flip(.05) {
		v := true
		// if flip(.5) {
		// 	v = false
		// }
		f.IPv6 = &v
	}
	if flip(.05) {
		v := true
		// currently, it's not checked against
		// if flip(.5) {
		// 	v = false
		// }
		f.Domain = &v
	}
	if flip(.05) {
		v := true
		if flip(.5) {
			v = false
		}
		f.Rentable = &v
	}
	if flip(.3) {
		c := agg.twins[rand.Intn(len(agg.twins))]
		if flip(.9) && len(agg.nodeRenters) != 0 {
			c = agg.nodeRenters[rand.Intn(len(agg.nodeRenters))]
		}
		f.RentedBy = &c
	}
	if flip(.3) {
		c := agg.twins[rand.Intn(len(agg.twins))]
		if flip(.1) && len(agg.nodeRenters) != 0 {
			c = agg.nodeRenters[rand.Intn(len(agg.nodeRenters))]
		}
		f.AvailableFor = &c
	}
	return f
}

func serializeFilter(f gridproxy.NodeFilter) string {
	res := ""
	if f.Status != nil {
		res = fmt.Sprintf("%sstatus: %s\n", res, *f.Status)
	}
	if f.FreeMRU != nil {
		res = fmt.Sprintf("%sFreeMRU: %d\n", res, *f.FreeMRU)
	}
	if f.FreeSRU != nil {
		res = fmt.Sprintf("%sFreeSRU: %d\n", res, *f.FreeSRU)
	}
	if f.FreeHRU != nil {
		res = fmt.Sprintf("%sFreeHRU: %d\n", res, *f.FreeHRU)
	}
	if f.Country != nil {
		res = fmt.Sprintf("%sCountry: %s\n", res, *f.Country)
	}
	if f.City != nil {
		res = fmt.Sprintf("%sCity: %s\n", res, *f.City)
	}
	if f.FarmName != nil {
		res = fmt.Sprintf("%sFarmName: %s\n", res, *f.FarmName)
	}
	if f.FarmIDs != nil {
		res = fmt.Sprintf("%sFarmIDs: %v\n", res, f.FarmIDs)
	}
	if f.FreeIPs != nil {
		res = fmt.Sprintf("%sFreeIPs: %d\n", res, *f.FreeIPs)
	}
	if f.IPv4 != nil {
		res = fmt.Sprintf("%sIPv4: %t\n", res, *f.IPv4)
	}
	if f.IPv6 != nil {
		res = fmt.Sprintf("%sIPv6: %t\n", res, *f.IPv6)
	}
	if f.Domain != nil {
		res = fmt.Sprintf("%sDomain: %t\n", res, *f.Domain)
	}
	if f.Rentable != nil {
		res = fmt.Sprintf("%sRentable: %t\n", res, *f.Rentable)
	}
	if f.Rentable != nil {
		res = fmt.Sprintf("%sRentable: %t\n", res, *f.Rentable)
	}
	if f.AvailableFor != nil {
		res = fmt.Sprintf("%sAvailableFor: %d\n", res, *f.AvailableFor)
	}
	return res
}

func NodeStressTest(data *DBData, proxyClient, localClient gridproxy.GridProxyClient) error {
	agg := calcNodesAggregates(data)
	for i := 0; i < Tests; i++ {
		l := gridproxy.Limit{
			Size:     999999999999,
			Page:     1,
			RetCount: false,
		}
		f := randomNodeFilter(&agg)

		localNodes, err := localClient.Nodes(f, l)
		if err != nil {
			return err
		}
		remoteNodes, err := proxyClient.Nodes(f, l)
		if err != nil {
			return err
		}
		returned[len(remoteNodes)] += 1
		if err := validateResults(localNodes, remoteNodes); err != nil {
			return errors.Wrapf(err, "filter: %s", serializeFilter(f))
		}

	}
	return nil
}

func NodesTest(data *DBData, proxyClient, localClient gridproxy.GridProxyClient) error {
	if err := NodeUpTest(data, proxyClient, localClient); err != nil {
		return err
	}
	if err := NodeStressTest(data, proxyClient, localClient); err != nil {
		return err
	}
	keys := make([]int, 0)
	for k, v := range returned {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Printf("(%d, %d)", k, returned[k])
	}
	return nil
}
