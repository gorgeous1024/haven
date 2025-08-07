package main

import (
	"context"
	"log/slog"

	"github.com/fiatjaf/khatru/blossom"
)

func migrateBlossomMetadata() {
	// Create a temporary Blossom instance for the migration
	blossomInstance := blossom.New(outboxRelay, "https://"+config.RelayURL)
	blossomInstance.Store = blossom.EventStoreBlobIndexWrapper{Store: blossomDB, ServiceURL: "https://" + config.RelayURL}
	outboxDBWrapper := blossom.EventStoreBlobIndexWrapper{Store: outboxDB, ServiceURL: "https://" + config.RelayURL}

	// List all BlobDescriptor for the relay owner pubkey
	ownerPubkey := nPubToPubkey(config.OwnerNpub)
	blobsChan, err := outboxDBWrapper.List(context.Background(), ownerPubkey)
	if err != nil {
		slog.Error("🚫 Failed to list blobs", "error", err)
		return
	}
	var blobs []blossom.BlobDescriptor
	for blob := range blobsChan {
		blobs = append(blobs, blob)
	}

	if len(blobs) == 0 {
		slog.Debug("No blobs found to migrate", "ownerPubkey", ownerPubkey)
		return
	}

	// Create a map to track migrated blobs
	migrated := make(map[string]blossom.BlobDescriptor, len(blobs))

	slog.Info("BlobDescriptors will be migrated from Outbox to Blossom's DB", "count", len(blobs))

	for _, blob := range blobs {
		slog.Debug("Moving BlobDescriptor", "sha256", blob.SHA256, "type", blob.Type, "size", blob.Size)

		if blob.Type == "" {
			blob.Type = "application/octet-stream"
		}

		err := blossomInstance.Store.Keep(context.Background(), blob, ownerPubkey)
		if err != nil {
			slog.Error("🚫 Failed to store blob in Blossom DB", "sha256", blob.SHA256, "error", err)
			continue
		}

		err = outboxDBWrapper.Delete(context.Background(), blob.SHA256, ownerPubkey)
		if err != nil {
			slog.Error("🚫 Failed to delete blob from outbox DB", "sha256", blob.SHA256, "error", err)
		}

		migrated[blob.SHA256] = blob
	}

	slog.Info("✅ Blob migration completed", "migrated", len(migrated))
}
