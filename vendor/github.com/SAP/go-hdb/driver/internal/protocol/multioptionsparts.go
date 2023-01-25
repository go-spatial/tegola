package protocol

import (
	"fmt"
	"sort"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

type topologyOption int8

const (
	toHostName         topologyOption = 1
	toHostPortnumber   topologyOption = 2
	toTenantName       topologyOption = 3
	toLoadfactor       topologyOption = 4
	toVolumeID         topologyOption = 5
	toIsPrimary        topologyOption = 6
	toIsCurrentSession topologyOption = 7
	toServiceType      topologyOption = 8
	toNetworkDomain    topologyOption = 9
	toIsStandby        topologyOption = 10
	toAllIPAddresses   topologyOption = 11
	toAllHostNames     topologyOption = 12
)

type topologyInformation []map[topologyOption]any

func (o topologyInformation) String() string {
	s1 := []string{}
	for _, ops := range o {
		s2 := []string{}
		for j, typ := range ops {
			s2 = append(s2, fmt.Sprintf("%s: %v", topologyOption(j), typ))
		}
		sort.Slice(s2, func(i, j int) bool { return s2[i] < s2[j] })
		s1 = append(s1, fmt.Sprintf("%v", s2))
	}
	return fmt.Sprintf("%v", s1)
}

func (o *topologyInformation) decode(dec *encoding.Decoder, ph *PartHeader) error {
	numArg := ph.numArg()
	*o = resizeSlice(*o, numArg)
	for i := 0; i < numArg; i++ {
		ops := map[topologyOption]any{}
		(*o)[i] = ops
		optCnt := int(dec.Int16())
		for j := 0; j < optCnt; j++ {
			k := topologyOption(dec.Int8())
			tc := typeCode(dec.Byte())
			ot := tc.optType()
			ops[k] = ot.decode(dec)
		}
	}
	return dec.Error()
}
