package aligner

import (
	"archive/zip"
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	logger "log"
	"os"
	"strings"

	"github.com/google/zopfli/go/zopfli"
)

type ZipAlign struct {
	check               bool
	overwrite           bool
	pageAlignSharedLibs bool
	verbose             bool
	zopfli              bool
	alignment           uint16
	newOffset           int64
}

func NewZipAlign(check, overwrite, pageAlignSharedLibs, verbose, zopfli bool, alignment uint16) *ZipAlign {
	zipAlign := ZipAlign{}
	zipAlign.check = check
	zipAlign.overwrite = overwrite
	zipAlign.pageAlignSharedLibs = pageAlignSharedLibs
	zipAlign.verbose = verbose
	zipAlign.zopfli = zopfli
	zipAlign.alignment = alignment
	return &zipAlign
}

func sizeofHeader(header *zip.FileHeader) int64 {
	length := int(30)
	length += len(header.Extra)
	length += len(header.Name)
	return int64(length)
}

func (zipalign *ZipAlign) CopyAndAlign(inputFile, outputFile string) int {
	// Open a zip archive for reading.
	zipReader, err := zip.OpenReader(inputFile)
	if err != nil {
		logger.Fatal(err)
	}
	defer zipReader.Close()

	buf, err := os.Create(outputFile)
	if err != nil {
		logger.Fatal(err)
	}
	defer buf.Close()

	// Create a new zip archive.
	zipWriter := zip.NewWriter(buf)
	defer zipWriter.Close()

	if zipalign.verbose {
		fmt.Printf("Aligning %q on %d bytes\n", inputFile, zipalign.alignment)
		fmt.Printf("writing out to %q begin...\n", outputFile)
	}

	// Iterate through the files in the archive,
	for _, entry := range zipReader.File {
		var newSize int64

		if entry.Method == zip.Deflate {
			// copy the entry without padding
			if zipalign.zopfli {
				newSize = zipalign.addRecompress(zipWriter, entry)
			} else {
				newSize = zipalign.add(zipWriter, entry, 1)
			}
		} else {
			alignTo := zipalign.getAlignment(entry.Name)
			newSize = zipalign.add(zipWriter, entry, alignTo)
		}
		zipalign.newOffset += newSize
	}

	if zipalign.verbose {
		fmt.Printf("writing out to %q succesful\n", outputFile)
	}
	return 0
}

func (zipalign *ZipAlign) addRecompress(w *zip.Writer, entry *zip.File) int64 {
	rc, err := entry.Open()
	if err != nil {
		logger.Fatal(err)
	}
	defer rc.Close()

	// uncompress
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, rc)
	if err != nil {
		logger.Fatal(entry.Name, ":", err)
	}
	data := zopfli.Deflate(buf.Bytes())
	length := len(data)
	fwhead := entry.FileHeader
	fwhead.CompressedSize = uint32(length)
	fwhead.CompressedSize64 = uint64(length)

	hash := crc32.NewIEEE()
	_, err = hash.Write(buf.Bytes())
	if err != nil {
		logger.Fatal(entry.Name, ":", err)
	}
	fwhead.CRC32 = hash.Sum32()

	fw, err := w.CreateRaw(&fwhead)
	if err != nil {
		logger.Fatal(entry.Name, ":", err)
	}

	n, err := io.Copy(fw, bytes.NewReader(data))
	if err != nil {
		logger.Fatal(entry.Name, ":", err)
	}

	return sizeofHeader(&fwhead) + n + 16
}

func (zipalign *ZipAlign) add(w *zip.Writer, entry *zip.File, alignTo uint16) int64 {
	fwhead := entry.FileHeader
	offsetHeader := sizeofHeader(&fwhead)
	padding := (alignTo - uint16((zipalign.newOffset+int64(offsetHeader))%int64(alignTo))) % alignTo

	//	HEADERFLAG  efHeaderID;
	//  ushort  efDataSize;
	//  char efData[ efDataSize ];
	//
	// add padding number of null bytes to the extra field of the file header
	// in order to align files on 4 bytes
	if padding > 0 {
		extra := []byte{'\x00', '\x00', '\x00', '\x00'}

		// reset the padding
		offsetHeader := sizeofHeader(&fwhead) + int64(len(extra))
		padding := (alignTo - uint16((zipalign.newOffset+int64(offsetHeader))%int64(alignTo))) % alignTo
		extra[2] = byte(padding)
		extra[3] = byte(padding >> 8)
		for i := uint16(0); i < padding; i++ {
			extra = append(extra, '\x00')
		}

		// append a new extraField
		fwhead.Extra = append(fwhead.Extra, extra...)
	}

	fw, err := w.CreateRaw(&fwhead)
	if err != nil {
		logger.Fatal(entry.Name, ":", err)
	}

	rc, err := entry.OpenRaw()
	if err != nil {
		logger.Fatal(err)
	}

	n, err := io.Copy(fw, rc)
	if err != nil {
		logger.Fatal(entry.Name, ":", err)
	}

	return sizeofHeader(&fwhead) + n + 16
}

func (zipalign *ZipAlign) getAlignment(name string) uint16 {
	const kPageAlignment = 4096

	if !zipalign.pageAlignSharedLibs {
		return zipalign.alignment
	}

	if strings.HasSuffix(name, ".so") {
		return kPageAlignment
	}

	return zipalign.alignment
}

func (zipalign *ZipAlign) Verify(fileName string) int {
	if zipalign.verbose {
		fmt.Printf("Verifying alignment of %s (%d)...\n", fileName, zipalign.alignment)
	}

	// Open a zip archive for reading.
	r, err := zip.OpenReader(fileName)
	if err != nil {
		logger.Fatal(err)
	}
	defer r.Close()

	foundBad := false
	for _, entry := range r.File {
		offset, _ := entry.DataOffset()

		if entry.Method == zip.Deflate {
			if zipalign.verbose {
				fmt.Printf("%8d %s (OK - compressed)\n", offset, entry.Name)
			}
		} else {
			alignTo := zipalign.getAlignment(entry.Name)
			if (offset % int64(alignTo)) != 0 {
				if zipalign.verbose {
					fmt.Printf("%8d %s (BAD - %d)\n", offset, entry.Name, offset%int64(alignTo))
				}
				foundBad = true
			} else {
				if zipalign.verbose {
					fmt.Printf("%8d %s (OK)\n", offset, entry.Name)
				}
			}
		}
	}

	if foundBad {
		if zipalign.verbose {
			fmt.Println("Verification", "FAILED")
		}
		return 1
	} else {
		if zipalign.verbose {
			fmt.Println("Verification", "succesful")
		}
		return 0
	}
}
