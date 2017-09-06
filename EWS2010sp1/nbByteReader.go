package EWS2010sp1

import (
	"bufio"
	"errors"
	"io"
	"time"
)

type nbByteReader struct {
	reader *bufio.Reader

	dataChan chan byte
	errChan  chan error
}

func newNBByteReader(r *bufio.Reader) *nbByteReader {
	return &nbByteReader{
		reader:   r,
		dataChan: make(chan byte),
		errChan:  make(chan error),
	}
}

func (r *nbByteReader) Dispose() {
	close(r.errChan)
	close(r.dataChan)
}

func (r *nbByteReader) TryReadByte(timeout time.Duration) (rv byte, isTimeout bool, err error) {
	go func() {
		defer func() {
			recover()
		}()

		nByte, err := r.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				r.errChan <- err
			}
		}
		r.dataChan <- nByte
	}()

	select {
	case nByte, ok := <-r.dataChan:
		if !ok {
			return 0, false, errors.New("Invalid operation on nbByteReader")
		}
		return nByte, false, nil
	case err, ok := <-r.errChan:
		if !ok {
			return 0, false, errors.New("Invalid operation on nbByteReader")
		}
		return 0, false, err
	case <-time.After(timeout):
		return 0, true, nil
	}

	return 0, false, errors.New("Invalid operation : nbByteReader disposed?")

}
