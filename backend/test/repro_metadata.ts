import { Connection, PublicKey } from '@solana/web3.js';
import * as splToken from '@solana/spl-token';
const RPC = 'https://api.mainnet-beta.solana.com';
const MINT = 'GeqaEdxaYCYnqgbrqGcCjjEtEJNcYZSKuJmR97zEpump';
async function main() {
  console.log(`Analyzing Mint: ${MINT}`);
  const connection = new Connection(RPC);
  const mintPubkey = new PublicKey(MINT);
  // 2. Check Token Program checks
  console.log('\n--- 2. Direct Account Info ---');
  const accountInfo = await connection.getAccountInfo(mintPubkey);
  if (!accountInfo) {
    console.log('Mint account not found');
    return;
  }
  console.log('Owner:', accountInfo.owner.toBase58());
  // Token2022 Program ID: TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb
  if (
    accountInfo.owner.toBase58() ===
    'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb'
  ) {
    console.log('Identified as Token 2022');
    try {
      const metadata = await splToken.getTokenMetadata(connection, mintPubkey);
      if (metadata) {
        console.log('Token 2022 Extension Metadata:', metadata);
      } else {
        console.log('No Token 2022 Metadata Extension found.');
      }
    } catch (e) {
      console.log('SPL Token Error:', e);
    }
  } else {
    console.log('Not Token 2022 (likely standard Token Program)');
  }
  // 3. Manual PDA Check
  console.log('\n--- 3. Manual Metadata PDA Check ---');
  const METADATA_PROGRAM_ID = new PublicKey(
    'metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s',
  );
  const [pda] = PublicKey.findProgramAddressSync(
    [
      Buffer.from('metadata'),
      METADATA_PROGRAM_ID.toBuffer(),
      mintPubkey.toBuffer(),
    ],
    METADATA_PROGRAM_ID,
  );
  console.log('PDA:', pda.toBase58());
  const pdaInfo = await connection.getAccountInfo(pda);
  if (pdaInfo) {
    console.log('PDA Account exists. Data length:', pdaInfo.data.length);

    // Manual Borsh Deserialization for Metadata
    try {
      let offset = 1 + 32 + 32; // key + updateAuthority + mint

      const nameLength = pdaInfo.data.readUInt32LE(offset);
      offset += 4;
      const name = pdaInfo.data
        .slice(offset, offset + nameLength)
        .toString('utf-8')
        .trim();
      offset += nameLength;

      const symbolLength = pdaInfo.data.readUInt32LE(offset);
      offset += 4;
      const symbol = pdaInfo.data
        .slice(offset, offset + symbolLength)
        .toString('utf-8')
        .trim();
      offset += symbolLength;

      const uriLength = pdaInfo.data.readUInt32LE(offset);
      offset += 4;
      const uri = pdaInfo.data
        .slice(offset, offset + uriLength)
        .toString('utf-8')
        .trim();

      console.log('Parsed Metadata:');
      console.log('Name:', name);
      console.log('Symbol:', symbol);
      console.log('URI:', uri);
    } catch (e) {
      console.log('Failed to parse PDA data:', e);
    }
  } else {
    console.log('PDA Account does NOT exist.');
  }
}
main();
