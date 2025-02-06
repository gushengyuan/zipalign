ZipAlign
========

A Go replacement for Android's SDK ZipAlign tool. See
https://developer.android.com/studio/command-line/zipalign.html for more
information.

When signing an APK with jarsigner, the returned zip is not aligned properly.
Android optimizes APK by aligning each file on 4 bytes to load them with mmap.
Running zipalign on an unaligned APK fixes its padding in the headers of each
entry to align it properly.

Example
-------

The command below takes a signed unaligned apk and fixes its padding. Verbosity
is reduced to only show files that require padding. The aligned APK is then
verified with Android's original zipalign tool to make sure the alignment is
correct per Android standard.

````
$ zipalign -v -f 4 ~/app-rocket-webkit-release-1816-signed.apk /tmp/rocket-aligned.signed.apk
···
Aligning "~/app-rocket-webkit-release-1816-signed.apk" on 4 bytes
writing out to "/tmp/rocket-aligned.signed.apk" begin...
writing out to "/tmp/rocket-aligned.signed.apk" succesful
Verifying alignment of /tmp/rocket-aligned.signed.apk (4)...
......
Verification succesful
···

$ /opt/android-sdk/build-tools/27.0.3/zipalign -c -v 4 /tmp/rocket-aligned.signed.apk
...
Verification succesful
````

Usage
-------

```
user@MacBook-Pro demo % ./zipalign
Zip alignment utility
Copyright (C) 2009 The Android Open Source Project

Usage: zipalign [-f] [-p] [-v] [-z] <align> infile.zip outfile.zip
       zipalign -c [-p] [-v] <align> infile.zip

  <align>: alignment in bytes, e.g. '4' provides 32-bit alignment
  -c: check alignment only (does not modify file)
  -f: overwrite existing outfile.zip
  -p: memory page alignment for stored shared object files
  -v: verbose output
  -z: recompress using Zopfli
user@MacBook-Pro demo % 
```

Reference
-------

```
Python: https://github.com/heisai/zipalign
Java: https://github.com/Iyxan23/zipalign-java
Android 12+ standalone: https://github.com/RohitVerma882/aapt2
Android 12: https://android.googlesource.com/platform/build.git/+/android-12.0.0_r34/tools/zipalign/ZipAlign.cpp
Android build tags: https://source.android.com/docs/setup/about/build-numbers#source-code-tags-and-builds
ZIP (file format): https://en.wikipedia.org/wiki/ZIP_(file_format)
Google zopfli: https://github.com/google/zopfli
go-zopfli: https://github.com/foobaz/go-zopfli
```

Trouble-shooting
-------

### problem: undefined reference to __imp___iob_func on Windows
```
PS ...\zipalign> go build
# .../zipalign
...\go\pkg\tool\windows_amd64\link.exe: running gcc failed: exit status 1
.../ld.exe: ...\AppData\Local\Temp\go-link-624120042\000005.o: in function `_cgo_preinit_init':
\\_\_\runtime\cgo/gcc_libinit_windows.c:30: undefined reference to `__imp___iob_func'
......
```

solution: force rebuilding of packages that are already up-to-date
```
go build -a
```


### problem: 'zopfli.h' file not found
```
# github.com/google/zopfli/go/zopfli
.../zopfli.go:23:10: fatal error: 'zopfli.h' file not found
#include "zopfli.h"
         ^~~~~~~~~~
1 error generated.
```

solution: add cgo CFLAGS in zopfli.go
```
#cgo CFLAGS: -I../../src/zopfli
```


### problem: library not found for -lzopfli
```
ld: library not found for -lzopfli
clang: error: linker command failed with exit code 1 (use -v to see invocation)
```

solution: add cgo LDFLAGS in zopfli.go
```
#cgo LDFLAGS: -lzopfli -lm -L../..
```

### CFLAGS and LDFLAGS doesn't work in zopfli.go
using -x option to see build details
```
go build -x
```

you will see the new CFLAGS and LDFLAGS are not passed to the compiler, so we should clean the build cache
```
go clean --cache
```

### problem: unknown option: -soname on macOS
```
ld: unknown option: -soname
clang: error: linker command failed with exit code 1 (use -v to see invocation)
```

solution: change -soname to -install_name in Makefile
```
sed -i "" 's/-soname/-install_name/g' Makefile
```

### how to build universal binary for macOS
append "-arch x86_64 -arch arm64" to CFLAGS and LDFLAGS in Makefile
```
CGO_ENABLED=1 GOARCH=arm64 go build -o zipalign-arm64
CGO_ENABLED=1 GOARCH=amd64 go build -o zipalign-x86_64
lipo -create -output zipalign -arch x86_64 zipalign-x86_64 -arch arm64 zipalign-arm64
```

be remember to set CGO_ENABLED=1 whle building zipalign, or there will be error like this
```
github.com/google/zopfli/go/zopfli: build constraints exclude all Go files in /Users/xxx/go/pkg/mod/github.com/google/zopfli@v0.0.0-20210614151705-831773bc28e3/go/zopfli
```

### be remember to implement Zlib and Deflate in zopfli.go
```
// Compresses data with Zopfli using default settings and zopfliFormat format.
// The Zopfli library does not return errors, and there are no (detectable)
// failure cases, hence no error return.
func compress(zopfliFormat uint32, inputSlice []byte) []byte {
	var options C.struct_ZopfliOptions
	C.ZopfliInitOptions(&options)

	inputSize := (C.size_t)(len(inputSlice))
	if inputSize == 0 {
		return []byte(emptyGzip)
	}
	input := (*C.uchar)(unsafe.Pointer(&inputSlice[0]))
	var compressed *C.uchar
	var compressedLength C.size_t

	C.ZopfliCompress(&options, C.ZopfliFormat(zopfliFormat),
		input, inputSize,
		&compressed, &compressedLength)
	defer C.free(unsafe.Pointer(compressed))

	// GoBytes only accepts int, not C.size_t. The code below does the same minus
	// protection against zero-length values, but compressedLength is never 0 due
	// to headers.
	result := make([]byte, compressedLength)
	C.memmove(unsafe.Pointer(&result[0]), unsafe.Pointer(compressed),
		compressedLength)
	return result
}

// Compresses data with Zopfli using default settings and gzip format.
// The Zopfli library does not return errors, and there are no (detectable)
// failure cases, hence no error return.
func Gzip(inputSlice []byte) []byte {
	return compress(C.ZOPFLI_FORMAT_GZIP, inputSlice)
}

// Compresses data with Zopfli using default settings and zlib format.
// The Zopfli library does not return errors, and there are no (detectable)
// failure cases, hence no error return.
func Zlib(inputSlice []byte) []byte {
	return compress(C.ZOPFLI_FORMAT_ZLIB, inputSlice)
}

// Compresses data with Zopfli using default settings and deflate format.
// The Zopfli library does not return errors, and there are no (detectable)
// failure cases, hence no error return.
func Deflate(inputSlice []byte) []byte {
	return compress(C.ZOPFLI_FORMAT_DEFLATE, inputSlice)
}
```
