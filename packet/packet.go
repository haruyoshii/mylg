// packet is a wrapper for GoPacket and sub packages
package packet

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// Packet holds all layers information
type Packet struct {
	Eth     *layers.Ethernet
	IPv4    *layers.IPv4
	IPv6    *layers.IPv6
	TCP     *layers.TCP
	UDP     *layers.UDP
	SrcHost []string
	DstHost []string
	Payload string
}

var (
	device            = "en0"
	snapLen     int32 = 1024
	promiscuous       = false
	err         error
	timeout     = 100 * time.Nanosecond
	handle      *pcap.Handle
)

// NewPacket creates an empty packet info
func NewPacket() *Packet {
	return &Packet{}
}

// Open is a loop over packets
func (p *Packet) Open() chan *Packet {
	var (
		c    = make(chan *Packet, 1)
		s    = make(chan os.Signal, 1)
		loop = true
	)
	// capture interrupt w/ s channel
	signal.Notify(s, os.Interrupt)

	go func() {
		handle, err = pcap.OpenLive(device, snapLen, promiscuous, timeout)
		if err != nil {
			log.Fatal(err)
		}
		if err := handle.SetBPFFilter(""); err != nil {
			log.Fatal(err)
		}

		defer handle.Close()
		defer close(s)
		defer close(c)

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for loop {
			packet, err := packetSource.NextPacket()
			if err != nil {
				continue
			}
			select {
			case <-s:
				loop = false
				signal.Stop(s)
			case c <- GetPacketInfo(packet):
			}
		}
	}()
	return c
}

// PrintPretty prints out the captured data
// to the stdout
func (p *Packet) PrintPretty() {
	switch p.Eth.EthernetType {
	case layers.EthernetTypeIPv4:
		p.PrintIPv4()
	case layers.EthernetTypeIPv6:
		println("IPV6")
		p.PrintIPv4()
	case layers.EthernetTypeARP:
		println("ARP")
	default:
		println("unknown")

	}
}

// PrintIPv4 prints IPv4 packets
func (p *Packet) PrintIPv4() {
	switch {
	case p.IPv4.Protocol == layers.IPProtocolTCP:
		log.Printf("IP %s > %s , %s length: %d\n", p.IPv4.SrcIP, p.IPv4.DstIP, p.IPv4.Protocol, len(p.Payload))
	case p.IPv4.Protocol == layers.IPProtocolUDP:
		log.Printf("IP %s > %s , %s length: %d\n", p.IPv4.SrcIP, p.IPv4.DstIP, p.IPv4.Protocol, len(p.Payload))
	}
}

// GetPacketInfo decodes layers
func GetPacketInfo(packet gopacket.Packet) *Packet {
	var p Packet
	// Ethernet
	ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	if ethernetLayer != nil {
		p.Eth, _ = ethernetLayer.(*layers.Ethernet)
	}

	// IP Address V4
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		p.IPv4, _ = ipLayer.(*layers.IPv4)
		p.SrcHost, _ = net.LookupHost(p.IPv4.SrcIP.String())
		p.DstHost, _ = net.LookupHost(p.IPv4.DstIP.String())
	} else {
		// IP Address V6
		ipLayer := packet.Layer(layers.LayerTypeIPv6)
		if ipLayer != nil {
			p.IPv6, _ = ipLayer.(*layers.IPv6)
			p.SrcHost, _ = net.LookupHost(p.IPv6.SrcIP.String())
			p.DstHost, _ = net.LookupHost(p.IPv6.DstIP.String())
		}
	}

	// TCP
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		p.TCP, _ = tcpLayer.(*layers.TCP)
	} else {
		// UDP
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		if udpLayer != nil {
			p.UDP, _ = udpLayer.(*layers.UDP)
		}
	}

	// Application
	applicationLayer := packet.ApplicationLayer()
	if applicationLayer != nil {
		p.Payload = string(applicationLayer.Payload())
	}

	// Check for errors
	if err := packet.ErrorLayer(); err != nil {
		fmt.Println("Error decoding some part of the packet:", err)
	}
	return &p
}