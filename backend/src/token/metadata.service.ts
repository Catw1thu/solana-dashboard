import { Injectable } from '@nestjs/common';
import { Connection, PublicKey } from '@solana/web3.js';
import { ConfigService } from '@nestjs/config';
import axios from 'axios';
import { DatabaseService } from '../database/database.service';
@Injectable()
export class MetadataService {
  private connection: Connection;
  private readonly METADATA_PROGRAM_ID = new PublicKey(
    'metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s',
  );
  constructor(
    private configService: ConfigService,
    private databaseService: DatabaseService,
  ) {
    const rpcUrl =
      this.configService.get<string>('RPC_URL') ||
      'https://api.mainnet-beta.solana.com';
    this.connection = new Connection(rpcUrl, 'confirmed');
  }
  async fetchAndSaveMetadata(poolAddress: string, mintAddress: string) {
    try {
      console.log(
        `Fetching metadata for mint: ${mintAddress} (Pool: ${poolAddress})`,
      );
      const metadata = await this.getMetadata(mintAddress);
      if (!metadata) {
        console.warn(`No metadata found for ${mintAddress}`);
        return;
      }
      let image = '';
      if (metadata.uri) {
        try {
          const { data } = await axios.get(metadata.uri, { timeout: 5000 });
          if (data && data.image) {
            image = data.image;
          }
        } catch (e) {
          console.warn(
            `Failed to fetch off-chain metadata from ${metadata.uri}: ${e.message}`,
          );
          // Fallback for IPFS
          if (metadata.uri.includes('/ipfs/')) {
            const ipfsHash = metadata.uri.split('/ipfs/')[1];
            if (ipfsHash) {
              const gateways = [
                'https://ipfs.io/ipfs/',
                'https://gateway.pinata.cloud/ipfs/',
                'https://cloudflare-ipfs.com/ipfs/',
              ];
              for (const gateway of gateways) {
                try {
                  const fallbackUrl = `${gateway}${ipfsHash}`;
                  console.log(`Trying fallback gateway: ${fallbackUrl}`);
                  const { data } = await axios.get(fallbackUrl, {
                    timeout: 5000,
                  });
                  if (data && data.image) {
                    image = data.image;
                    console.log(`✅ Recovered metadata from ${gateway}`);
                    break;
                  }
                } catch (retryError) {
                  continue;
                }
              }
            }
          }
        }
      }
      await this.databaseService.updatePoolMetadata(poolAddress, {
        name: metadata.name.replace(/\0/g, ''),
        symbol: metadata.symbol.replace(/\0/g, ''),
        image: image,
      });
      console.log(
        `✅ Metadata saved: ${metadata.symbol} | ${image ? 'Has Image' : 'No Image'}`,
      );
    } catch (e) {
      console.error(
        `Failed to process metadata for ${mintAddress}: ${e.message}`,
      );
    }
  }
  private async getMetadata(
    mint: string,
  ): Promise<{ name: string; symbol: string; uri: string } | null> {
    try {
      const mintPubkey = new PublicKey(mint);
      const [pda] = PublicKey.findProgramAddressSync(
        [
          Buffer.from('metadata'),
          this.METADATA_PROGRAM_ID.toBuffer(),
          mintPubkey.toBuffer(),
        ],
        this.METADATA_PROGRAM_ID,
      );
      const accountInfo = await this.connection.getAccountInfo(pda);
      if (!accountInfo) return null;
      // Manual Borsh Deserialization for Metadata
      // First byte is discriminator
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
      return { name, symbol, uri };
    } catch (e) {
      console.error(`Error parsing on-chain metadata: ${e.message}`);
      return null;
    }
  }
}
