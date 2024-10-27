# goShrinkMyEth


Shrinking the Blockchain - making ETH archive small finally.

Found this challenge for the compasslabs bounty at the 2024 Encode London hackathon. https://compasslabs.notion.site/Shrink-The-Chain-Challenge-1270e31195818067b5bee42fb09aa629

Requirements
The problem to solve it is the fact that only the whole chain is stored when you have an archive node, while you might only need a subset of the data.

You might be interested in just a full history for a couple of accounts, but the usual operation of the nodes - either Anvil or Erigon - still requires you to sync all. This can take even a couple of days!

The bounty states to cache only txns that a given address took part in, or an array of addresses, with an extended logic option for all transactions within n degrees of proximity to the origin address?

The whole archive at the moment can be about 4TB of data. It’d be way less if we were just scanning a handful of addresses!

Then you might think that it’s the network part that makes it take so long. I thought so at first.

Turns out, it’s the local revalidation that is the culprit. The synching of the erigon node happens in 12 phases and validation is the fourth one.

I spoke with the Compass Labs team and the key JSON RPC methods for the MVP that they were interested in were

eth_getStorage
eth_getCode
eth_getTransaction - by hash, blockhash and indexer, block number and index, receipt
With that in mind, I set out to solve the bounty.

Two strategies
The bounty description mentions two ways of adding the desired functionality. Either a pull request to an existing Ethereum client, such as Erigon or Anvil, or to make a new service. I explored both in parallel.

The resources provided included ssh access to a server with an archive node, so that I could inspect the archiving format. There in /mnt/erigon_data/snapshots I found the Ethereum blockchain history. That is in the format of files like: v1-012000-012500-headers.seg, which is generically v1-[startBlockNumber]-[endBlockNumber]-[headers/bodies/transactions].[seg/seg.torrent].

I heard that it might be a good idea to consider a forking mode, where a low-resource node is spun up, not doing staking nor validation, just copying the values of a different node. I wasn’t sure if that functionality was supported in Erigon node, but it’s present in the Anvil forking mode.

Then I dug the Erigon codebase a bit more and I learned that it is not supported there at the moment. Pivot time.

SCP-Copying
The next idea was to use the provided archive node to just copy the data and later parse it. I tried it from the ssh server with one v1-[...]-transactions.seg file, and then sought to decode it. I realized as the format is Erigon specific, using a B+tree key value store implementation, made specifically for Erigon. This meant I couldn’t easily use third party tools to unpack the seg files. If that was possible, I could do the ‘copy over ssh’, with the go-scp library.

I turned to trying to use the erigon library itself to accomplish the decompression and to readout the transactions.

Decompression
The labrynth codebase of erigon is over 5 years old and has thousands of files, with many features like an embedded database with complex byte level manipulation, support for multiple networking protocols and less documentation than I would have imagined.

First false lead I got was looking at the Tx interface, which turned out to be a database interface rather than the blockchain data type - that one is Txn in the Erigon world.

Next I looked into the backend.go file and TxPool, but that contains only the transactions to go into the next block, not past blocks.

Around this time I realized that this might be a bigger project than I expected. A much bigger node speedup challenge is underway in the Cardano ecosystem, and that is expected to take two years - the daedalus-turbo project.

I saw that I needed more understanding. I read through a substack article from the Erigon team describing the phases that happen during sync.

It can take many hours before control returns to the Headers stage again, by which time there are more headers available, so the process repeats, but with much fewer number of headers, and then blocks. Eventually, these repetitions converge to processing 1 (or sometimes more) blocks at a time, as they are being produced by the network.

I was planning to import part of the Erigon monorepo, wanting to copy the logic for stagesync - stages 1,2,3, 5 - rejecting the compute intensive stage 4. Still it was nebulous how to actually extract the transactions.

Alternatively I could manually run network json-rpc requests to the arbitum api to get the archived node data, but that would be quite network intensive.

Then I turned again to the documentation of Erigon, the db_walkthrough.md, to be precise. Going through the entire programmers_guide would be a good idea if I had more time.

Solution
Thankfully I discovered that Erigon interface provide a way to request data from a running node. That is defined in a separate repository for interfaces. This made me realize this is much more reachable than I thought.

SQL structures
Regarding the I immediately turned to pocketbase for persistence of read out data. You can define a schema and there the addresses and extracted txns could live.

My implementation data flow steps
This is not necessarily the best implementation order. I am also assuming that a full archive node is available to query with gRPC - and the provided arbitrum node is only set for JSON-RPC.

extract the TXNs from config
use gRPC to get the txns data
save to pocketbase db - https://pocketbase.io/docs/use-as-framework/
dump db contents on a custom RPC call https://github.com/pocketbase/pocketbase/discussions/115
serve the json rpc for the pocketbase backend - the custom ones https://pkg.go.dev/golang.org/x/exp/jsonrpc2
add custom indexes on pocketbase to increase read performance - by block https://github.com/pocketbase/pocketbase/issues/1466
all the other rpcs - just proxy to the upstream server
config supplied with a list of addresses
the database and json rpc get started
json rpc for provides a ‘syncing status’ endpoint to indicate syncing status
grpc client is initalized with a list of addresses
grpc client calls the full archive node for txn data
txn data is archived into the database
txn data is available for arbitrary clients
Try out the mermaid js editors here

graph TD
  A[User] -->|Supported RPC Call| B[JSON-RPC server]
  A -->|Unsupported RPC Call| G[Full Node]

  B -->|Database Read| C[Database]
  B -->|Database Sync| E[PocketBase]
  C -->|Dump DB to File| F[Debug Access over SSH]

  D[gRPC Client] -->|gRPC Call for Txn Data| G
  D -->|Database Sync| C
  E -->|Database Read| C
Sequence diagram:


sequenceDiagram
    participant main as main
    participant FullNode as Full Node
    participant GrpcClient as gRPC Client
    participant DB as Database
    participant JsonRpc as JSON-RPC Server
    participant User as User
    participant admin as admin
    participant fs as filesystem
    
    %% Step 1
    Note over main: 1. Config supplied with a list of addresses
    
    %% Step 2
    main ->> JsonRpc: 2. JSON-RPC starts

    %% Step 3
    main ->> DB : 2. Database starts

    %% Step 4
    User ->> JsonRpc: 3. Syncing status query
    JsonRpc -->> User: Syncing status response
    
    %% Step 5
    main ->> GrpcClient: 4. Initialize gRPC Client with addresses
    
    %% Step 6
    GrpcClient ->> FullNode: 5. gRPC call for transaction data
    FullNode -->> GrpcClient: Transaction data response
    
    %% Step 7
    GrpcClient ->> DB: 6. Archive transaction data in Database
    
    %% Step 8
    DB -->> JsonRpc: 7. Data available for client queries
    User ->> JsonRpc: JSON-RPC call for data
    JsonRpc -->> User: JSON-RPC data response

    %% Step 9 
    main -->> DB: 8. Tell DB to dump contents
    DB -->> fs: Save the data to the filesystem( sqlite3 dump)


    %% Step 10
    admin -->> fs: 9. Inspect the data for debugging
tables

addresses get storage get code
table addresses account - code - storage
table txns - txn - data structure
Implemntation is set in the goShrinkMyEth repo, in a not necessarily complete state.

thanks
Thanks to Ben and Lucas from the Compass Labs team for advice.

Annex
Reference json rpc explanations
https://docs.alchemy.com/reference/eth-getstorageat https://docs.alchemy.com/reference/eth-getcode https://docs.alchemy.com/reference/eth-gettransactionbyblockhashandindex https://docs.alchemy.com/reference/eth-gettransactionbyhash https://docs.alchemy.com/reference/eth-gettransactionbyblockhashandindex
