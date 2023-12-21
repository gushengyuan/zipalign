package main

import (
	"flag"
	"fmt"
	logger "log"
	"os"
	"strconv"

	"github.com/quark/zipalign/aligner"
)

func usage() {
	fmt.Println("Zip alignment utility")
	fmt.Println("Copyright (C) 2022 The Quark Open Source Project")
	fmt.Println()
	fmt.Println("Usage: zipalign [-f] [-p] [-v] [-z] <align> infile.zip outfile.zip")
	fmt.Println("       zipalign -c [-p] [-v] <align> infile.zip")
	fmt.Println()
	fmt.Println("  <align>: alignment in bytes, e.g. '4' provides 32-bit alignment")
	fmt.Println("  -c: check alignment only (does not modify file)")
	fmt.Println("  -f: overwrite existing outfile.zip")
	fmt.Println("  -p: memory page alignment for stored shared object files")
	fmt.Println("  -v: verbose output")
	fmt.Println("  -z: recompress using Zopfli")
}

func main() {
	var check = flag.Bool("c", false, "check alignment only (does not modify file)")
	var overwrite = flag.Bool("f", false, "overwrite existing outfile.zip")
	var help = flag.Bool("h", false, "print this help")
	var pageAlignSharedLibs = flag.Bool("p", false, "memory page alignment for stored shared object files")
	var verbose = flag.Bool("v", false, "verbose output")
	var zopfli = flag.Bool("z", false, "recompress using Zopfli")
	flag.Parse()

	// disable the logger prefix
	logger.SetFlags(0)

	var alignment int
	var inputFile string
	var outputFile string

	list := flag.Args()
	if len(list) < 2 {
		usage()
		os.Exit(1)
	}

	// get the default alignment
	alignment, err := strconv.Atoi(list[0])
	if err != nil {
		logger.Fatal("invalid alignment:", list[0])
	}

	zipalign := aligner.NewZipAlign(*check, *overwrite, *pageAlignSharedLibs, *verbose, *zopfli, uint16(alignment))
	if *check {
		inputFile = list[1]
		rc := zipalign.Verify(inputFile)
		os.Exit(rc)
	} else {
		if len(list) == 2 {
			inputFile = list[1]
		} else if len(list) == 3 {
			inputFile = list[1]
			outputFile = list[2]
		}
	}

	if inputFile == "" || outputFile == "" || *help {
		usage()
		os.Exit(1)
	}

	if inputFile == outputFile {
		logger.Fatalf("Input and output can't be same file")
	}

	_, err = os.Stat(outputFile)
	if err == nil && !*overwrite {
		logger.Fatalf("Output file '%s' exists", outputFile)
	}

	rc := zipalign.CopyAndAlign(inputFile, outputFile)
	if rc == 0 {
		rc = zipalign.Verify(outputFile)
		os.Exit(rc)
	}
}
