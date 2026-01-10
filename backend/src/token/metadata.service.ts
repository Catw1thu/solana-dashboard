import { Injectable, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { Connection, PublicKey } from '@solana/web3.js';
import { ConfigService } from '@nestjs/config';
import axios from 'axios';
import { DatabaseService } from '../database/database.service';
import { getTokenMetadata } from '@solana/spl-token';
import { Subject, Subscription } from 'rxjs';
import { mergeMap } from 'rxjs/operators';

@Injectable()
export class MetadataService implements OnModuleInit, OnModuleDestroy {
  private connection: Connection;
  private readonly metadataQueue = new Subject<{
    poolAddress: string;
    mintAddress: string;
  }>();
  private queueSubscription: Subscription;

  constructor(
    private configService: ConfigService,
    private databaseService: DatabaseService,
  ) {
    const rpcUrl =
      this.configService.get<string>('RPC_URL') ||
      'https://api.mainnet-beta.solana.com';
    this.connection = new Connection(rpcUrl, 'confirmed');
  }

  onModuleInit() {
    this.startQueueProcessor();
  }

  onModuleDestroy() {
    if (this.queueSubscription) {
      this.queueSubscription.unsubscribe();
    }
  }

  /**
   * Public method to queue a metadata fetch task.
   * Non-blocking (fire and forget).
   */
  queueFetch(poolAddress: string, mintAddress: string) {
    this.metadataQueue.next({ poolAddress, mintAddress });
  }

  private startQueueProcessor() {
    this.queueSubscription = this.metadataQueue
      .pipe(
        mergeMap(
          async (task) => await this.processMetadata(task),
          5, // Concurrency limit: 5 concurrent requests
        ),
      )
      .subscribe();
  }

  private async processMetadata({
    poolAddress,
    mintAddress,
  }: {
    poolAddress: string;
    mintAddress: string;
  }) {
    try {
      console.log(
        `[Metadata] Processing: ${mintAddress} (Pool: ${poolAddress})`,
      );

      let metadata: { name: string; symbol: string; uri: string } | null = null;
      let image = '';

      // 1. Try Token 2022 Extension (Store on Mint Account)
      try {
        const tokenMetadata = await getTokenMetadata(
          this.connection,
          new PublicKey(mintAddress),
        );

        if (tokenMetadata) {
          metadata = {
            name: tokenMetadata.name,
            symbol: tokenMetadata.symbol,
            uri: tokenMetadata.uri,
          };
        }
      } catch (e) {
        // Ignore, can happen if not Token 2022 or no extension
      }

      // 2. If not found, try Metaplex PDA (Legacy/Standard)
      if (!metadata) {
        try {
          const METADATA_PROGRAM_ID = new PublicKey(
            'metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s',
          );
          const mintPubkey = new PublicKey(mintAddress);
          const [pda] = PublicKey.findProgramAddressSync(
            [
              Buffer.from('metadata'),
              METADATA_PROGRAM_ID.toBuffer(),
              mintPubkey.toBuffer(),
            ],
            METADATA_PROGRAM_ID,
          );

          const accountInfo = await this.connection.getAccountInfo(pda);
          if (accountInfo) {
            // Manual Borsh Deserialization
            let offset = 1 + 32 + 32; // key + updateAuthority + mint

            const nameLength = accountInfo.data.readUInt32LE(offset);
            offset += 4;
            const name = accountInfo.data
              .slice(offset, offset + nameLength)
              .toString('utf-8')
              .trim();
            offset += nameLength;

            const symbolLength = accountInfo.data.readUInt32LE(offset);
            offset += 4;
            const symbol = accountInfo.data
              .slice(offset, offset + symbolLength)
              .toString('utf-8')
              .trim();
            offset += symbolLength;

            const uriLength = accountInfo.data.readUInt32LE(offset);
            offset += 4;
            const uri = accountInfo.data
              .slice(offset, offset + uriLength)
              .toString('utf-8')
              .trim();

            metadata = { name, symbol, uri: uri.replace(/\0/g, '') };
          }
        } catch (e) {
          console.warn(
            `[Metadata] PDA check failed for ${mintAddress}: ${e.message}`,
          );
        }
      }

      if (!metadata) {
        console.warn(`[Metadata] No metadata found for ${mintAddress}`);
        return;
      }

      // Fetch off-chain JSON
      if (metadata.uri) {
        image = await this.fetchOffChainMetadata(metadata.uri);
      }

      // Update Database
      await this.databaseService.updatePoolMetadata(poolAddress, {
        name: metadata.name.replace(/\0/g, ''),
        symbol: metadata.symbol.replace(/\0/g, ''),
        image: image,
      });

      console.log(
        `✅ [Metadata] Saved: ${metadata.symbol} | ${
          image ? 'Has Image' : 'No Image'
        }`,
      );
    } catch (e) {
      console.error(`❌ [Metadata] Failed for ${mintAddress}: ${e.message}`);
    }
  }

  private async fetchOffChainMetadata(uri: string): Promise<string> {
    try {
      // 1. Try direct fetch
      const { data } = await axios.get(uri, { timeout: 5000 });
      if (data && data.image) {
        return data.image;
      }
    } catch (e) {
      console.warn(`[Metadata] Off-chain fetch failed ${uri}: ${e.message}`);
    }

    // 2. IPFS Fallback
    if (uri.includes('/ipfs/')) {
      const ipfsHash = uri.split('/ipfs/')[1];
      if (ipfsHash) {
        const gateways = [
          'https://ipfs.io/ipfs/',
          'https://gateway.pinata.cloud/ipfs/',
          'https://cloudflare-ipfs.com/ipfs/',
        ];
        for (const gateway of gateways) {
          try {
            const fallbackUrl = `${gateway}${ipfsHash}`;
            // console.log(`Trying fallback: ${fallbackUrl}`);
            const { data } = await axios.get(fallbackUrl, {
              timeout: 5000,
            });
            if (data && data.image) {
              console.log(`✅ [Metadata] Recovered from ${gateway}`);
              return data.image;
            }
          } catch (retryError) {
            continue;
          }
        }
      }
    }
    return '';
  }
}
