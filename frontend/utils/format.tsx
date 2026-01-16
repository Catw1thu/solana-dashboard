export const formatAddress = (address: string) => {
  if (!address) return "";
  return `${address.slice(0, 4)}...${address.slice(-4)}`;
};

export const formatPrice = (price: number) => {
  if (!price) return "0.00";

  // Convert to string to check for leading zeros
  const priceStr = price.toFixed(18).replace(/\.?0+$/, "");

  if (price >= 0.01) {
    return price.toFixed(4); // Standard formatting for "normal" prices
  }

  // Count leading zeros after decimal point
  const match = price.toFixed(18).match(/^0\.0+/);
  if (!match) return price.toFixed(6);

  const zeroCount = match[0].length - 2; // -2 for "0."
  const significantDigits = priceStr.slice(
    match[0].length,
    match[0].length + 4
  );

  // Subscript mapping
  const subscriptMap: { [key: string]: string } = {
    "0": "₀",
    "1": "₁",
    "2": "₂",
    "3": "₃",
    "4": "₄",
    "5": "₅",
    "6": "₆",
    "7": "₇",
    "8": "₈",
    "9": "₉",
  };

  const subscript = zeroCount
    .toString()
    .split("")
    .map((c) => subscriptMap[c])
    .join("");

  return `0.0${subscript}${significantDigits}`;
};

export const formatAmount = (amount: number) => {
  if (!amount) return "0";

  if (amount >= 1_000_000_000) {
    return (amount / 1_000_000_000).toFixed(2) + "B";
  }
  if (amount >= 1_000_000) {
    return (amount / 1_000_000).toFixed(2) + "M";
  }
  if (amount >= 1_000) {
    return (amount / 1_000).toFixed(2) + "K";
  }

  return amount.toFixed(2);
};
