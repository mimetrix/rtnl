package rtnl

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

type IProute2Link struct {
	Link   string
	Ifname string
}

func Test_AddVeth(t *testing.T) {

	// add a veth link
	ve := &Link{
		Info: &LinkInfo{
			Name: "vethA",
			Veth: &Veth{
				Peer: "vethB",
			},
		},
	}
	err := ve.Add()
	if err != nil {
		t.Fatal(err)
	}
	if ve.Info.Veth.Peer != "vethB" {
		t.Fatal("peer lost")
	}

	// ensure iproute2 sees it and parameters are correct
	out, err := exec.Command(
		"ip", "-j", "link", "show", "dev", "vethA",
	).CombinedOutput()
	if err != nil {
		t.Fatal(string(out))
	}

	var ilinks []IProute2Link
	err = json.Unmarshal(out, &ilinks)
	if err != nil {
		t.Fatal(err)
	}

	if len(ilinks) == 0 {
		t.Fatal("created veth not found")
	}

	if ilinks[0].Ifname != "vethA" {
		t.Fatal("veth does not have correct name")
	}

	if ilinks[0].Link != "vethB" {
		t.Fatal("veth peer does not have correct name")
	}

	// read back and ensure peer equality

	lnk, err := GetLink("vethA")
	if ve.Info.Veth.Peer != lnk.Info.Veth.Peer {
		t.Fatalf("peer of read link not correct %v != %v",
			ve.Info.Veth.Peer, lnk.Info.Veth.Peer,
		)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = ve.Present()
	if err != nil {
		t.Fatal(err)
	}

	err = ve.Del()
	if err != nil {
		t.Fatal(err)
	}

	err = ve.Absent()
	if err != nil {
		t.Fatal(err)
	}

}

func Test_VethNamespace(t *testing.T) {

	va := &Link{
		Info: &LinkInfo{
			Name: "vethA",
			Veth: &Veth{
				Peer: "vethB",
			},
		},
	}
	err := va.Add()
	if err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("ip", "netns", "add", "pizza").CombinedOutput()
	if err != nil {
		t.Fatal(string(out))
	}

	f, err := os.Open("/var/run/netns/pizza")
	if err != nil {
		t.Fatalf("failed to open netns file: %v", err)
	}
	nsfd := f.Fd()

	vb := &Link{
		Info: &LinkInfo{
			Name: "vethB",
			Ns:   uint32(nsfd),
		},
	}
	err = vb.Set()
	if err != nil {
		t.Fatal(err)
	}

	// ensure iproute2 sees the link in correc namespace
	out, err = exec.Command(
		"ip", "netns", "exec", "pizza", "ip", "-j", "link", "show", "dev", "vethB",
	).CombinedOutput()
	if err != nil {
		t.Fatal(string(out))
	}

	var ilinks []IProute2Link
	err = json.Unmarshal(out, &ilinks)
	if err != nil {
		t.Fatal(err)
	}

	if len(ilinks) == 0 {
		t.Fatal("created veth not found")
	}

	if ilinks[0].Ifname != "vethB" {
		t.Fatal("veth does not have correct name")
	}

	// note that we cannot test for the peer here because it is in a different
	// namespace

	// cleanup

	va.Del()

	out, err = exec.Command("ip", "netns", "del", "pizza").CombinedOutput()
	if err != nil {
		t.Fatal(string(out))
	}

}

func Test_VethAddress(t *testing.T) {

	va := &Link{
		Info: &LinkInfo{
			Name: "vethA",
			Veth: &Veth{
				Peer: "vethB",
			},
		},
	}
	err := va.Add()
	if err != nil {
		t.Fatal(err)
	}

	addr, err := ParseAddr("192.168.47.1/24")
	if err != nil {
		t.Fatal(err)
	}

	err = va.AddAddr(addr)
	if err != nil {
		t.Fatal(err)
	}

	err = va.Up()
	if err != nil {
		t.Fatal(err)
	}

	vb, err := GetLink("vethB")
	if err != nil {
		t.Fatal(err)
	}

	err = vb.Up()
	if err != nil {
		t.Fatal(err)
	}

	err = va.Del()
	if err != nil {
		t.Fatal(err)
	}

}

func Test_Bridge(t *testing.T) {

	br := &Link{
		Info: &LinkInfo{
			Name:   "br47",
			Bridge: &Bridge{},
		},
	}
	err := br.Present()
	if err != nil {
		t.Fatal(err)
	}

	addr, _ := ParseAddr("1.2.3.4/24")
	err = br.AddAddr(addr)
	if err != nil {
		br.Del()
		t.Fatal(err)
	}

	va := &Link{
		Info: &LinkInfo{
			Name:   "vethA",
			Master: uint32(br.Msg.Index),
			Veth: &Veth{
				Peer: "vethB",
			},
		},
	}
	err = va.Add()
	if err != nil {
		t.Fatal(err)
	}

	err = va.Del()
	if err != nil {
		t.Fatal(err)
	}

	err = br.Del()
	if err != nil {
		t.Fatal(err)
	}

}

func Test_Vxlan(t *testing.T) {

	lo, err := GetLink("lo")
	if err != nil {
		t.Fatal(err)
	}

	vx := &Link{
		Info: &LinkInfo{
			Name: "vtep47",
			Vxlan: &Vxlan{
				Vni:     47,
				DstPort: 4789,
				Local:   net.ParseIP("1.2.3.4"),
				Link:    uint32(lo.Msg.Index),
			},
		},
	}

	err = vx.Add()
	if err != nil {
		t.Fatal(err)
	}

	xv, err := GetLink("vtep47")
	if err != nil {
		t.Error(err)
	}
	if err == nil {
		if xv.Info.Vxlan == nil {
			t.Error("no vxlan data")
		}
		if !reflect.DeepEqual(*xv.Info.Vxlan, *vx.Info.Vxlan) {
			t.Error("vxlans do not match")
			t.Logf("expected: %#v", *vx.Info.Vxlan)
			t.Logf("actual: %#v", *xv.Info.Vxlan)
		}
	}

	err = vx.Del()
	if err != nil {
		t.Fatal(err)
	}

}
