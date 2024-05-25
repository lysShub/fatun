package pcap

import (
	"context"

	"github.com/lysShub/fatun"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
)

type CaptureWrapper struct {
	fatun.Capturer
	pcap *pcap.Pcap
}

func WrapCapture(capturer fatun.Capturer, file string) fatun.Capturer {
	p, err := pcap.File(file)
	if err != nil {
		panic(err)
	}
	return &CaptureWrapper{
		Capturer: capturer,
		pcap:     p,
	}
}

func (c *CaptureWrapper) Capture(ctx context.Context, ip *packet.Packet) error {
	err := c.Capturer.Capture(ctx, ip)
	if err != nil {
		return err
	}
	return c.pcap.WriteIP(ip.Bytes())
}
func (c *CaptureWrapper) Inject(ctx context.Context, ip *packet.Packet) error {
	err := c.Capturer.Inject(ctx, ip)
	if err != nil {
		return err
	}
	return c.pcap.WriteIP(ip.Bytes())
}
func (c *CaptureWrapper) Close() error {
	defer func() { c.pcap.Close() }()
	return c.Capturer.Close()
}
func (c *CaptureWrapper) Unwrap() fatun.Capturer { return c.Capturer }

type SenderWrapper struct {
	fatun.Sender
	pcap *pcap.Pcap
}

func (s *SenderWrapper) Send(ctx context.Context, ip *packet.Packet) error {
	err := s.Sender.Send(ctx, ip)
	if err != nil {
		return err
	}
	return s.pcap.WriteIP(ip.Bytes())
}
func (s *SenderWrapper) Recv(ctx context.Context, ip *packet.Packet) error {
	err := s.Sender.Recv(ctx, ip)
	if err != nil {
		return err
	}
	return s.pcap.WriteIP(ip.Bytes())
}
func (s *SenderWrapper) Close() error {
	defer func() { s.pcap.Close() }()
	return s.Sender.Close()
}
func (c *SenderWrapper) Unwrap() fatun.Sender { return c.Sender }
