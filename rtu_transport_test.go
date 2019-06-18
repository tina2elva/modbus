package modbus

import (
	"testing"
	"io"
	"net"
	"time"
)

func TestAssembleRTUFrame(t *testing.T) {
	var rt		*rtuTransport
	var frame	[]byte

	rt		= newRTUTransport(&ClientConfiguration{})

	frame		= rt.assembleRTUFrame(&request{
		unitId:		0x33,
		functionCode:	0x11,
		payload:	[]byte{0x22, 0x33, 0x44, 0x55},
	})
	// expect 1 byte of unit id, 1 byte of function code, 4 bytes of payload and
	// 2 bytes of CRC
	if len(frame) != 8 {
		t.Errorf("expected 8 bytes, got %v", len(frame))
	}
	for i, b := range []byte{
		0x33, 0x11, // unit id and function code
		0x22, 0x33, // payload
		0x44, 0x55, // payload
		0xf0, 0x93, // CRC
	} {
		if frame[i] != b {
			t.Errorf("expected 0x%02x at position %v, got 0x%02x", b, i, frame[i])
		}
	}

	frame		= rt.assembleRTUFrame(&request{
		unitId:		0x31,
		functionCode:	0x06,
		payload:	[]byte{0x12, 0x34},
	})
	// expect 1 byte of unit if, 1 byte of function code, 2 bytes of payload and
	// 2 bytes of CRC
	if len(frame) != 6 {
		t.Errorf("expected 6 bytes, got %v", len(frame))
	}
	for i, b := range []byte{
		0x31, 0x06, // unit id and function code
		0x12, 0x34, // payload
		0xe3, 0xae, // CRC
	} {
		if frame[i] != b {
			t.Errorf("expected 0x%02x at position %v, got 0x%02x", b, i, frame[i])
		}
	}

	return
}


func TestRTUTransportReadResponse(t *testing.T) {
	var rt		*rtuTransport
	var p1, p2	net.Conn
	var txchan	chan []byte
	var err		error
	var res		*response

	txchan		= make(chan []byte, 2)
	p1, p2		= net.Pipe()
	go feedTestPipe(t, txchan, p1)


	rt		= newRTUTransport(&ClientConfiguration{
		Timeout: 10 * time.Millisecond,
	})
	rt.port		= p2

	// read a valid response (illegal data address)
	txchan		<- []byte{
		0x31, 0x82, // unit id and response code
		0x02,       // exception code
		0xc1, 0x6e, // CRC
	}
	res, err	= rt.ReadResponse()
	if err != nil {
		t.Errorf("ReadResponse() should have succeeded, got %v", err)
	}
	if res.unitId != 0x31 {
		t.Errorf("expected 0x31 as unit id, got 0x%02x", res.unitId)
	}
	if res.responseCode != 0x82 {
		t.Errorf("expected 0x82 as function code, got 0x%02x", res.responseCode)
	}
	if len(res.payload) != 1 {
		t.Errorf("expected a length of 1, got %v", len(res.payload))
	}
	if res.payload[0] != 0x02 {
		t.Errorf("expected {0x02} as payload, got {0x%02x}",
			 res.payload[0])
	}

	// read a frame with a bad crc
	txchan		<- []byte{
		0x30, 0x82, // unit id and response code
		0x12,       // exception code
		0xc0, 0xa2, // CRC
	}
	res, err	= rt.ReadResponse()
	if err != ErrBadCRC {
		t.Errorf("ReadResponse() should have returned ErrBadCrc, got %v", err)
	}

	// read a longer, valid response
	txchan		<- []byte{
		0x31, 0x03, // unit id and response code
		0x04,       // length
		0x11, 0x22, // register #1
		0x33, 0x44, // register #2
		0x7b, 0xc5, // CRC
	}
	res, err	= rt.ReadResponse()
	if err != nil {
		t.Errorf("ReadResponse() should have succeeded, got %v", err)
	}
	if res.unitId != 0x31 {
		t.Errorf("expected 0x31 as unit id, got 0x%02x", res.unitId)
	}
	if res.responseCode != 0x03 {
		t.Errorf("expected 0x03 as function code, got 0x%02x", res.responseCode)
	}
	if len(res.payload) != 5 {
		t.Errorf("expected a length of 5, got %v", len(res.payload))
	}
	for i, b := range []byte{
		0x04,
		0x11, 0x22,
		0x33, 0x44,
	} {
		if res.payload[i] != b {
			t.Errorf("expected 0x%02x at position %v, got 0x%02x",
				 b, i, res.payload[i])
		}
	}

	p1.Close()
	p2.Close()

	return
}

func feedTestPipe(t *testing.T, in chan []byte, out io.WriteCloser) {
	var err		error
	var txbuf	[]byte

	for {
		// grab a slice of bytes from the channel
		txbuf = <-in

		// write this slice to the pipe
		_, err = out.Write(txbuf)
		if err != nil {
			t.Errorf("failed to write to test pipe: %v", err)
			return
		}
	}

	return
}