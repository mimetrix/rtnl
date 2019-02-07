package rtnl

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

var ctx = &Context{Ns: 0}

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
	err := ve.Add(ctx)
	t.Logf("%+v", ve.Info)
	if err != nil {
		t.Fatal(err)
	}
	ve.Info.Veth.ResolvePeer(ctx)
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

	lnk, err := GetLink(ctx, "vethA")
	t.Logf("%+v", lnk.Info)
	lnk.Info.Veth.ResolvePeer(ctx)
	if ve.Info.Veth.Peer != lnk.Info.Veth.Peer {
		t.Fatalf("peer of read link not correct %v != %v",
			ve.Info.Veth.Peer, lnk.Info.Veth.Peer,
		)
	}
	if ve.Info.Address.String() != lnk.Info.Address.String() {
		t.Fatalf("L2 address mismatch %s != %s",
			ve.Info.Address.String(),
			lnk.Info.Address.String(),
		)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = ve.Present(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = ve.Del(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = ve.Absent(ctx)
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
	err := va.Add(ctx)
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
	err = vb.Set(ctx)
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

	va.Del(ctx)

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
	err := va.Add(ctx)
	if err != nil {
		t.Fatal(err)
	}

	addr, err := ParseAddr("192.168.47.1/24")
	if err != nil {
		t.Fatal(err)
	}

	err = va.AddAddr(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}

	err = va.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}

	vb, err := GetLink(ctx, "vethB")
	if err != nil {
		t.Fatal(err)
	}

	err = vb.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = va.Del(ctx)
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
	err := br.Present(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = br.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}

	br.Info.Promisc = true
	err = br.Set(ctx)
	if err != nil {
		t.Fatal(err)
	}

	brr, err := GetLink(ctx, "br47")
	if err != nil {
		t.Fatal(err)
	}

	if !brr.Info.Promisc {
		br.Del(ctx)
		t.Fatal("promisc readback failed")
	}

	addr, _ := ParseAddr("1.2.3.4/24")
	err = br.AddAddr(ctx, addr)
	if err != nil {
		br.Del(ctx)
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
	err = va.Add(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = va.Del(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = br.Del(ctx)
	if err != nil {
		t.Fatal(err)
	}

}

func Test_Vxlan(t *testing.T) {

	lo, err := GetLink(ctx, "lo")
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

	err = vx.Add(ctx)
	if err != nil {
		t.Fatal(err)
	}

	xv, err := GetLink(ctx, "vtep47")
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

	err = vx.Del(ctx)
	if err != nil {
		t.Fatal(err)
	}

}
