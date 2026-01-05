-- CreateTable
CREATE TABLE "Pool" (
    "address" TEXT NOT NULL,
    "baseMint" TEXT NOT NULL,
    "quoteMint" TEXT NOT NULL,
    "baseDecimals" INTEGER NOT NULL,
    "quoteDecimals" INTEGER NOT NULL,

    CONSTRAINT "Pool_pkey" PRIMARY KEY ("address")
);

-- CreateTable
CREATE TABLE "Trade" (
    "time" TIMESTAMP(3) NOT NULL,
    "txHash" TEXT NOT NULL,
    "poolAddress" TEXT NOT NULL,
    "type" TEXT NOT NULL,
    "price" DOUBLE PRECISION NOT NULL,
    "baseAmount" DOUBLE PRECISION NOT NULL,
    "quoteAmount" DOUBLE PRECISION NOT NULL,

    CONSTRAINT "Trade_pkey" PRIMARY KEY ("time","txHash")
);

-- CreateIndex
CREATE INDEX "Trade_poolAddress_time_idx" ON "Trade"("poolAddress", "time" DESC);

-- AddForeignKey
ALTER TABLE "Trade" ADD CONSTRAINT "Trade_poolAddress_fkey" FOREIGN KEY ("poolAddress") REFERENCES "Pool"("address") ON DELETE RESTRICT ON UPDATE CASCADE;
