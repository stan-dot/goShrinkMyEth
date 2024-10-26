
package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"github.com/erigontech/interfaces"

)

// TxnData represents transaction data for caching
type TxnData struct {
	ID        uint64 `gorm:"primaryKey"`
	Key       []byte
	Value     []byte
	Timestamp time.Time
}

// KVClient manages the gRPC client connection and the database connection
type KVClient struct {
	client remote.KVClient
	db     *gorm.DB
}

// NewKVClient initializes a new KVClient with gRPC and database connections
func NewKVClient(grpcAddr string) (*KVClient, error) {
	// Establish gRPC connection
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := remote.NewKVClient(conn)

	// Set up SQLite database connection
	db, err := gorm.Open(sqlite.Open("transactions.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Migrate the schema for TxnData
	if err := db.AutoMigrate(&TxnData{}); err != nil {
		return nil, err
	}

	return &KVClient{client: client, db: db}, nil
}

// CacheTransactions retrieves transactions via gRPC and stores them in the database
func (kv *KVClient) CacheTransactions(ctx context.Context) error {
	stream, err := kv.client.Tx(ctx, &remote.Cursor{})
	if err != nil {
		return err
	}

	for {
		// Receive data from the stream
		pair, err := stream.Recv()
		if err != nil {
			log.Printf("Error receiving stream: %v", err)
			break
		}

		// Create a TxnData instance and save to database
		txnData := TxnData{
			ID:        pair.TxId,
			Key:       pair.K,
			Value:     pair.V,
			Timestamp: time.Now(),
		}

		if err := kv.db.Create(&txnData).Error; err != nil {
			log.Printf("Error saving transaction to DB: %v", err)
		}
	}

	return nil
}

func main() {
	// Create context with timeout for gRPC connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Initialize KVClient
	kvClient, err := NewKVClient("localhost:50051") // Adjust to the gRPC server address
	if err != nil {
		log.Fatalf("Failed to initialize KV client: %v", err)
	}

	// Fetch and cache transactions
	if err := kvClient.CacheTransactions(ctx); err != nil {
		log.Fatalf("Failed to cache transactions: %v", err)
	}

	log.Println("Transaction caching complete.")
}
