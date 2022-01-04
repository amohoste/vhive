package thindelta

import (
	"encoding/gob"
	"github.com/pkg/errors"
	"os"
)

// BlockDelta Stores the block difference between two snapshot devices.
type BlockDelta struct {
	DiffBlocks *[]DiffBlock
	BlockSizeBytes int64
}

// DiffBlock represent a contiguous set of Length physical blocks starting at block Begin on disk that differ between
// two devices. If blocks have not been deleted in the second device, the bytes contained in the block are stored in
// the Bytes array.
type DiffBlock struct {
	Begin int64
	Length int64
	Delete bool
	Bytes []byte
}

// NewBlockDelta initializes a new BlockDelta to store the block difference between two snapshot devices.
func NewBlockDelta(diffBlocks *[]DiffBlock, blockSizeBytes int64) *BlockDelta {
	blockDelta := new(BlockDelta)
	blockDelta.DiffBlocks = diffBlocks
	blockDelta.BlockSizeBytes = blockSizeBytes
	return blockDelta
}

// Serialize serializes the difference between two snapshots to disk. This could be used to implement remote
// snapshotting  if snapshots of the same image are deterministically flattened into a file system.
func (bld *BlockDelta) Serialize(storePath string) error {
	file, err := os.Create(storePath)
	if err != nil {
		return errors.Wrapf(err, "creating block delta file")
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)

	err = encoder.Encode(*bld.DiffBlocks)
	if err != nil {
		return errors.Wrapf(err, "encoding blocks delta")
	}
	return nil
}

// DeserializeDiffBlocks deserializes the difference between two snapshots from disk. BlockDelta can be initialized
// as an empty array before using.
func (bld *BlockDelta) DeserializeDiffBlocks(storePath string) error {
	file, err := os.Open(storePath)
	if err != nil {
		return errors.Wrapf(err, "opening block delta file")
	}
	defer file.Close()

	encoder := gob.NewDecoder(file)

	err = encoder.Decode(bld.DiffBlocks)
	if err != nil {
		return errors.Wrapf(err, "decoding block delta")
	}
	return nil
}

// ReadBlocks directly reads the computed differing blocks from the specified data device.
func (bld *BlockDelta) ReadBlocks(dataDevPath string) error {
	file, err := os.Open(dataDevPath)
	defer file.Close()

	if err != nil {
		return errors.Wrapf(err, "opening data device for reading")
	}

	for idx, diffBlock := range *bld.DiffBlocks {
		if ! diffBlock.Delete {
			toRead := diffBlock.Length * bld.BlockSizeBytes

			buf := make([]byte, toRead)
			offset := diffBlock.Begin * bld.BlockSizeBytes

			bytesRead, err := file.ReadAt(buf, offset)
			if err != nil {
				return errors.Wrapf(err, "reading snapshot blocks")
			}

			if bytesRead != int(toRead) {
				return errors.New("Read less bytes than requested. This should not happen")
			}
			(*bld.DiffBlocks)[idx].Bytes = buf
		}
	}
	return nil
}

// WriteBlocks directly writes the differing blocks to the specified destination data device.
func (bld *BlockDelta) WriteBlocks(dataDevPath string) error {
	file, err := os.OpenFile(dataDevPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer file.Close()

	if err != nil {
		return errors.Wrapf(err, "opening data device for writing")
	}

	for _, diffBlock := range *bld.DiffBlocks {
		toWrite := diffBlock.Length * bld.BlockSizeBytes

		var buf []byte
		if ! diffBlock.Delete {
			buf = diffBlock.Bytes
		} else {
			// If delete, write 0 bytes. Could be done more optimally
			buf = make([]byte, toWrite)
		}

		offset := diffBlock.Begin * bld.BlockSizeBytes

		bytesWritten, err := file.WriteAt(buf, offset)
		if err != nil {
			return errors.Wrapf(err, "writing snapshot blocks")
		}

		if bytesWritten != int(toWrite) {
			return errors.New("Wrote less bytes than requested. This should not happen")
		}
	}
	return nil
}