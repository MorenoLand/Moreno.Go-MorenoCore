package mpq

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoshVarga/blast"
)

func TestDecompressImplode(t *testing.T) {
	var compressed bytes.Buffer
	writer := blast.NewWriter(&compressed, blast.Binary, blast.DictionarySize1024)
	if _, err := writer.Write([]byte("legacy MPQ implode data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	want := []byte("legacy MPQ implode data")
	decoded, err := decompress(compressed.Bytes(), uint32(len(want)), fileImplode)
	if err != nil || !bytes.Equal(decoded, want) {
		t.Fatalf("decoded=%q err=%v", decoded, err)
	}
	multi := append([]byte{0x08}, compressed.Bytes()...)
	decoded, err = decompress(multi, uint32(len(want)), fileCompress)
	if err != nil || !bytes.Equal(decoded, want) {
		t.Fatalf("multi decoded=%q err=%v", decoded, err)
	}
}

func TestDecompressBzip2Sector(t *testing.T) {
	compressed, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWUT3E3gAAAGRgEAABkSQgCAAIgM0hDAhtoFUJ4u5IpwoSCJ7ibwA")
	if err != nil {
		t.Fatal(err)
	}
	compressed = append([]byte{0x10}, compressed...)
	decoded, err := decompress(compressed, uint32(len("hello world")), fileCompress)
	if err != nil || !bytes.Equal(decoded, []byte("hello world")) {
		t.Fatalf("decoded=%q err=%v", decoded, err)
	}
}

func TestDecompressWaveADPCM(t *testing.T) {
	mono := append([]byte{0x40}, 0, 0, 0, 0, 0)
	decoded, err := decompress(mono, 4, fileCompress)
	if err != nil || len(decoded) != 4 || decoded[0] != 0 || decoded[1] != 0 {
		t.Fatalf("mono decoded=%x err=%v", decoded, err)
	}
	stereo := append([]byte{0x80}, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	decoded, err = decompress(stereo, 8, fileCompress)
	if err != nil || len(decoded) != 8 {
		t.Fatalf("stereo decoded=%x err=%v", decoded, err)
	}
}

func TestDecompressCombinedZlibPKWare(t *testing.T) {
	want := []byte("combined MPQ compression")
	var pkware bytes.Buffer
	writer := blast.NewWriter(&pkware, blast.Binary, blast.DictionarySize1024)
	if _, err := writer.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var zlibData bytes.Buffer
	zlibWriter := zlib.NewWriter(&zlibData)
	if _, err := zlibWriter.Write(pkware.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zlibWriter.Close(); err != nil {
		t.Fatal(err)
	}
	combined := append([]byte{0x0A}, zlibData.Bytes()...)
	decoded, err := decompress(combined, uint32(len(want)), fileCompress)
	if err != nil || !bytes.Equal(decoded, want) {
		t.Fatalf("decoded=%q err=%v", decoded, err)
	}
}

func TestDecompressHuffman(t *testing.T) {
	for _, dataType := range []uint32{1, 2, 3} {
		want := []byte("MPQ adaptive Huffman fixture")
		encoded, err := Compress(want, dataType)
		if err != nil {
			t.Fatalf("data type %d encode: %v", dataType, err)
		}
		sector := append([]byte{0x01}, encoded...)
		decoded, err := decompress(sector, uint32(len(want)), fileCompress)
		if err != nil || !bytes.Equal(decoded, want) {
			t.Fatalf("data type %d decoded=%q err=%v", dataType, decoded, err)
		}
	}
}

func TestDecompressSparse(t *testing.T) {
	compressed := []byte{0, 0, 0, 7, 0x82, 'a', 'b', 'c', 0x00, 0x80, 'z'}
	want := []byte{'a', 'b', 'c', 0, 0, 0, 'z'}
	decoded, err := decompress(append([]byte{0x20}, compressed...), uint32(len(want)), fileCompress)
	if err != nil || !bytes.Equal(decoded, want) {
		t.Fatalf("decoded=%x err=%v", decoded, err)
	}
	if _, err := decompress([]byte{0x20, 0, 0, 0, 8, 0x82, 'a'}, 8, fileCompress); err == nil {
		t.Fatal("expected truncated sparse stream error")
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"foo\\bar.dbc", "FOO/BAR.DBC"},
		{"FOO//BAR.DBC", "FOO/BAR.DBC"},
		{"a\\b\\c.mpq", "A/B/C.MPQ"},
	}
	for _, tc := range cases {
		actual := normalize(tc.input)
		if actual != tc.expected {
			t.Errorf("normalize(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestHashStringDeterministic(t *testing.T) {
	h1 := hashString("Interface/FrameXML/UI.xml", 0)
	h2 := hashString("Interface/FrameXML/UI.xml", 0)
	if h1 != h2 {
		t.Fatalf("expected deterministic hashString, got %x vs %x", h1, h2)
	}
	if h1 == 0 {
		t.Fatal("expected non-zero hash")
	}

	hType1 := hashString("test", 1)
	hType2 := hashString("test", 2)
	if hType1 == hType2 {
		t.Fatal("expected different hashes for different hashTypes")
	}
}

func TestArchivesDiscovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpq_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy .mpq files
	f1 := filepath.Join(tempDir, "common.mpq")
	f2 := filepath.Join(tempDir, "patch.mpq")
	_ = os.WriteFile(f1, []byte("dummy1"), 0o644)
	_ = os.WriteFile(f2, []byte("dummy2"), 0o644)

	archives, err := Archives(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 2 {
		t.Fatalf("expected 2 archives, got %d", len(archives))
	}
}

func TestOpenInvalidFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpq_open_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	invalidFile := filepath.Join(tempDir, "invalid.mpq")
	_ = os.WriteFile(invalidFile, []byte("not an mpq file"), 0o644)

	_, err = Open(invalidFile)
	if err == nil {
		t.Fatal("expected error opening non-MPQ file")
	}
}

func TestReadHeaderScansEmbeddedVersionOneArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embedded.mpq")
	data := make([]byte, 512+44)
	offset := 512
	binary.LittleEndian.PutUint32(data[offset:], archiveMagic)
	binary.LittleEndian.PutUint32(data[offset+4:], 44)
	binary.LittleEndian.PutUint32(data[offset+8:], 4096)
	binary.LittleEndian.PutUint16(data[offset+12:], 1)
	binary.LittleEndian.PutUint16(data[offset+14:], 3)
	binary.LittleEndian.PutUint32(data[offset+16:], 0x100)
	binary.LittleEndian.PutUint32(data[offset+20:], 0x200)
	binary.LittleEndian.PutUint32(data[offset+24:], 16)
	binary.LittleEndian.PutUint32(data[offset+28:], 8)
	binary.LittleEndian.PutUint64(data[offset+32:], 0x300)
	binary.LittleEndian.PutUint16(data[offset+40:], 1)
	binary.LittleEndian.PutUint16(data[offset+42:], 2)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	archive := &Archive{path: path, file: file}
	if err := archive.readHeader(); err != nil {
		t.Fatal(err)
	}
	if archive.header.ArchiveOffset != 512 || archive.header.HashTablePos != (1<<32)+0x100+512 || archive.header.BlockTablePos != (2<<32)+0x200+512 || archive.header.ExtendedBlockTable != 0x300+512 {
		t.Fatalf("header offsets archive=%d hash=%d block=%d extended=%d", archive.header.ArchiveOffset, archive.header.HashTablePos, archive.header.BlockTablePos, archive.header.ExtendedBlockTable)
	}
}

func encryptMPQFixture(data []byte, key uint32) {
	cryptOnce.Do(initCryptTable)
	seed := key
	seed2 := uint32(0xEEEEEEEE)
	for offset := 0; offset+4 <= len(data); offset += 4 {
		seed2 += cryptTable[0x400+(seed&0xFF)]
		plain := binary.LittleEndian.Uint32(data[offset:])
		cipher := plain ^ (seed + seed2)
		seed = ((^seed << 21) + 0x11111111) | (seed >> 11)
		seed2 = plain + seed2 + (seed2 << 5) + 3
		binary.LittleEndian.PutUint32(data[offset:], cipher)
	}
}

func writeEmbeddedMPQFixture(t *testing.T, name string, payload []byte, single, encrypted bool) string {
	t.Helper()
	archiveOffset := 512
	hashOffset, blockOffset, extendedOffset, fileOffset := 0x100, 0x200, 0x300, 0x400
	blockSize := uint32(512 << 3)
	fileData := append([]byte(nil), payload...)
	if !single {
		sectorCount := (uint32(len(payload)) + blockSize - 1) / blockSize
		tableSize := (sectorCount + 1) * 4
		fileData = make([]byte, int(tableSize)+len(payload))
		for index := uint32(0); index <= sectorCount; index++ {
			offset := uint32(tableSize)
			if index < sectorCount {
				offset += index * blockSize
			} else {
				offset += uint32(len(payload))
			}
			binary.LittleEndian.PutUint32(fileData[index*4:], offset)
		}
		copy(fileData[tableSize:], payload)
		if encrypted {
			encryptMPQFixture(fileData[:tableSize], hashString(name, 3)-1)
			for index := uint32(0); index < sectorCount; index++ {
				start := uint32(tableSize) + index*blockSize
				end := start + blockSize
				if end > uint32(len(fileData)) {
					end = uint32(len(fileData))
				}
				encryptMPQFixture(fileData[start:end], hashString(name, 3)+index)
			}
		}
	} else if encrypted {
		encryptMPQFixture(fileData, hashString(name, 3))
	}
	data := make([]byte, archiveOffset+fileOffset+len(fileData))
	header := data[archiveOffset:]
	binary.LittleEndian.PutUint32(header[0:], archiveMagic)
	binary.LittleEndian.PutUint32(header[4:], 44)
	binary.LittleEndian.PutUint32(header[8:], uint32(fileOffset+len(fileData)))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 3)
	binary.LittleEndian.PutUint32(header[16:], uint32(hashOffset))
	binary.LittleEndian.PutUint32(header[20:], uint32(blockOffset))
	binary.LittleEndian.PutUint32(header[24:], 16)
	binary.LittleEndian.PutUint32(header[28:], 1)
	binary.LittleEndian.PutUint64(header[32:], uint64(extendedOffset))
	hashes := data[archiveOffset+hashOffset : archiveOffset+hashOffset+16*16]
	for index := 0; index < 16; index++ {
		binary.LittleEndian.PutUint32(hashes[index*16:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(hashes[index*16+4:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(hashes[index*16+12:], 0xFFFFFFFF)
	}
	hashSlot := int(hashString(name, 0) % 16)
	binary.LittleEndian.PutUint32(hashes[hashSlot*16:], hashString(name, 1))
	binary.LittleEndian.PutUint32(hashes[hashSlot*16+4:], hashString(name, 2))
	binary.LittleEndian.PutUint32(hashes[hashSlot*16+12:], 0)
	encryptMPQFixture(hashes, hashString("(hash table)", 3))
	blocks := data[archiveOffset+blockOffset : archiveOffset+blockOffset+16]
	binary.LittleEndian.PutUint32(blocks[0:], uint32(fileOffset))
	binary.LittleEndian.PutUint32(blocks[4:], uint32(len(fileData)))
	binary.LittleEndian.PutUint32(blocks[8:], uint32(len(payload)))
	flags := fileExists
	if single {
		flags |= fileSingle
	}
	if encrypted {
		flags |= fileEncrypt
	}
	binary.LittleEndian.PutUint32(blocks[12:], flags)
	encryptMPQFixture(blocks, hashString("(block table)", 3))
	copy(data[archiveOffset+extendedOffset:], []byte{0, 0})
	copy(data[archiveOffset+fileOffset:], fileData)
	path := filepath.Join(t.TempDir(), "fixture.mpq")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadEncryptedMultiSectorFile(t *testing.T) {
	payload := make([]byte, 5000)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	path := writeEmbeddedMPQFixture(t, "Data/sector.bin", payload, false, true)
	archive, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	got, err := archive.ReadFile("Data/sector.bin")
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("encrypted multi-sector read len=%d err=%v", len(got), err)
	}
}

func writeEmbeddedMPQZlibFixture(t *testing.T, name string, payload []byte) string {
	t.Helper()
	archiveOffset := 512
	hashOffset, blockOffset, extendedOffset, fileOffset := 0x100, 0x200, 0x300, 0x400
	blockSize := uint32(512 << 3)
	sectorCount := (uint32(len(payload)) + blockSize - 1) / blockSize
	encoded := make([][]byte, sectorCount)
	for index := uint32(0); index < sectorCount; index++ {
		start := index * blockSize
		end := start + blockSize
		if end > uint32(len(payload)) {
			end = uint32(len(payload))
		}
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		if _, err := writer.Write(payload[start:end]); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		encoded[index] = append([]byte{0x02}, compressed.Bytes()...)
	}
	tableSize := (sectorCount + 1) * 4
	fileData := make([]byte, tableSize)
	dataOffset := tableSize
	for index, sector := range encoded {
		binary.LittleEndian.PutUint32(fileData[uint32(index)*4:], uint32(dataOffset))
		fileData = append(fileData, sector...)
		dataOffset += uint32(len(sector))
	}
	binary.LittleEndian.PutUint32(fileData[sectorCount*4:], uint32(dataOffset))
	data := make([]byte, archiveOffset+fileOffset+len(fileData))
	header := data[archiveOffset:]
	binary.LittleEndian.PutUint32(header[0:], archiveMagic)
	binary.LittleEndian.PutUint32(header[4:], 44)
	binary.LittleEndian.PutUint32(header[8:], uint32(fileOffset+len(fileData)))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 3)
	binary.LittleEndian.PutUint32(header[16:], uint32(hashOffset))
	binary.LittleEndian.PutUint32(header[20:], uint32(blockOffset))
	binary.LittleEndian.PutUint32(header[24:], 16)
	binary.LittleEndian.PutUint32(header[28:], 1)
	binary.LittleEndian.PutUint64(header[32:], uint64(extendedOffset))
	hashes := data[archiveOffset+hashOffset : archiveOffset+hashOffset+16*16]
	for index := 0; index < 16; index++ {
		binary.LittleEndian.PutUint32(hashes[index*16:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(hashes[index*16+4:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(hashes[index*16+12:], 0xFFFFFFFF)
	}
	hashSlot := int(hashString(name, 0) % 16)
	binary.LittleEndian.PutUint32(hashes[hashSlot*16:], hashString(name, 1))
	binary.LittleEndian.PutUint32(hashes[hashSlot*16+4:], hashString(name, 2))
	binary.LittleEndian.PutUint32(hashes[hashSlot*16+12:], 0)
	encryptMPQFixture(hashes, hashString("(hash table)", 3))
	blocks := data[archiveOffset+blockOffset : archiveOffset+blockOffset+16]
	binary.LittleEndian.PutUint32(blocks[0:], uint32(fileOffset))
	binary.LittleEndian.PutUint32(blocks[4:], uint32(len(fileData)))
	binary.LittleEndian.PutUint32(blocks[8:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(blocks[12:], fileExists|fileCompress)
	encryptMPQFixture(blocks, hashString("(block table)", 3))
	copy(data[archiveOffset+extendedOffset:], []byte{0, 0})
	copy(data[archiveOffset+fileOffset:], fileData)
	path := filepath.Join(t.TempDir(), "zlib-sectors.mpq")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadCompressedMultiSectorFile(t *testing.T) {
	payload := bytes.Repeat([]byte("compressed MPQ sector parity "), 400)
	path := writeEmbeddedMPQZlibFixture(t, "Data/zlib.bin", payload)
	archive, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	got, err := archive.ReadFile("Data/zlib.bin")
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("compressed multi-sector read len=%d err=%v", len(got), err)
	}
}

func TestOpenReadsEmbeddedVersionOneArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.mpq")
	archiveOffset := 512
	hashOffset, blockOffset, extendedOffset, fileOffset := 0x100, 0x200, 0x300, 0x400
	listfile := []byte("Interface/Test.txt\r\n")
	data := make([]byte, archiveOffset+fileOffset+len(listfile))
	header := data[archiveOffset:]
	binary.LittleEndian.PutUint32(header[0:], archiveMagic)
	binary.LittleEndian.PutUint32(header[4:], 44)
	binary.LittleEndian.PutUint32(header[8:], uint32(fileOffset+len(listfile)))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 3)
	binary.LittleEndian.PutUint32(header[16:], uint32(hashOffset))
	binary.LittleEndian.PutUint32(header[20:], uint32(blockOffset))
	binary.LittleEndian.PutUint32(header[24:], 16)
	binary.LittleEndian.PutUint32(header[28:], 1)
	binary.LittleEndian.PutUint64(header[32:], uint64(extendedOffset))
	hashes := data[archiveOffset+hashOffset : archiveOffset+hashOffset+16*16]
	for index := 0; index < 16; index++ {
		binary.LittleEndian.PutUint32(hashes[index*16:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(hashes[index*16+4:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(hashes[index*16+12:], 0xFFFFFFFF)
	}
	listHash := hashString("(listfile)", 0)
	hashSlot := int(listHash % 16)
	binary.LittleEndian.PutUint32(hashes[hashSlot*16:], hashString("(listfile)", 1))
	binary.LittleEndian.PutUint32(hashes[hashSlot*16+4:], hashString("(listfile)", 2))
	binary.LittleEndian.PutUint16(hashes[hashSlot*16+8:], 0)
	binary.LittleEndian.PutUint16(hashes[hashSlot*16+10:], 0)
	binary.LittleEndian.PutUint32(hashes[hashSlot*16+12:], 0)
	encryptMPQFixture(hashes, hashString("(hash table)", 3))
	blocks := data[archiveOffset+blockOffset : archiveOffset+blockOffset+16]
	binary.LittleEndian.PutUint32(blocks[0:], uint32(fileOffset))
	binary.LittleEndian.PutUint32(blocks[4:], uint32(len(listfile)))
	binary.LittleEndian.PutUint32(blocks[8:], uint32(len(listfile)))
	binary.LittleEndian.PutUint32(blocks[12:], fileExists|fileSingle|fileEncrypt)
	encryptMPQFixture(blocks, hashString("(block table)", 3))
	copy(data[archiveOffset+extendedOffset:], []byte{0, 0})
	encryptedListfile := append([]byte(nil), listfile...)
	encryptMPQFixture(encryptedListfile, hashString("(listfile)", 3))
	copy(data[archiveOffset+fileOffset:], encryptedListfile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	archive, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files, err := archive.ListFiles()
	if err != nil || len(files) != 1 || files[0] != "Interface/Test.txt" {
		t.Fatalf("listfile=%v err=%v", files, err)
	}
}
