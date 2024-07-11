package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

type Pipe struct {
	outputPath string
	reader     func() (io.ReadCloser, error)
	writers    []func() (io.WriteCloser, error)
}

func InitPipe(outputPath string) *Pipe {
	return &Pipe{
		outputPath: outputPath,
		reader: func() (io.ReadCloser, error) {
			fmt.Println("opening reader" + outputPath)
			reader, err := os.OpenFile(outputPath, os.O_RDONLY, 0)
			fmt.Println("opened reader" + outputPath)
			return reader, err
		},
	}
}
func (p *Pipe) addWriter(writer func() (io.WriteCloser, error)) {
	p.writers = append(p.writers, writer)
}
func (p *Pipe) Run() {
	var err error
	r, err := p.reader()
	if err != nil {
		log.Fatalf("Error:%v", err)
	}
	log.Printf("Reader get:%v", r)
	ws := make([]io.WriteCloser, len(p.writers))
	for i, w := range p.writers {
		w, err := w()
		if err != nil {
			log.Fatalf("Error:%v", err)
		}
		log.Printf("Writer get:%v", w)
		defer w.Close()
		ws[i] = w
	}
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			for _, w := range ws {
				_, err := w.Write(buf[:n])
				if err != nil {
					log.Fatalf("Error:%v", err)
				}
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Fatalf("%v", err)
			}
			break
		}
	}
	log.Printf("Finished")
}
