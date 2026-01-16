export const formatAddress = (address: string) => {
  if (!address) return "";
  return `${address.slice(0, 4)}...${address.slice(-4)}`;
};

export const formatPrice = (originalPrice: number) => {
  if (!originalPrice) return "0.00";

  // 1. 处理负数情况，确保逻辑只处理正数
  const price = Math.abs(originalPrice);

  // 2. 如果价格大于等于 0.01，直接显示标准格式
  if (price >= 0.01) {
    return originalPrice.toFixed(4);
  }

  // 3. 使用 Math.log10 计算前导零的个数
  // 例如: 0.000001 (10^-6) -> log10是-6 -> -(-6) - 1 = 5 个零
  const zeroCount = -Math.floor(Math.log10(price)) - 1;

  // 保护机制：如果零特别少（比如 0.009），可能不需要下标，视需求而定
  // 这里保留你原本的逻辑：只要小于 0.01 就开始用下标

  // 4. 获取有效数字
  // 将数字转为字符串，截取"0."和"前导零"之后的部分
  // toFixed(18) 保证精度足够，substring 跳过 "0." (2个字符) + zeroCount 个零
  const significantDigits = price
    .toFixed(18)
    .substring(2 + zeroCount)
    .slice(0, 4); // 取4位有效数字

  // 5. 下标映射
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
    .map((c) => subscriptMap[c] || c) //虽然理论上不会超出，但加个fallback
    .join("");

  // 6. 组合结果 (如果有负号，加回去)
  const sign = originalPrice < 0 ? "-" : "";

  // 处理特殊情况：如果有非常多的零导致计算异常，回退显示
  if (zeroCount < 2) {
    // 0.00123 这种情况，zeroCount是2，可能你想显示 0.0012 而不是 0.0₂12
    // 如果你想对 0.00xx 也用普通显示，可以在这里加判断
    // return originalPrice.toFixed(6);
  }

  return `${sign}0.0${subscript}${significantDigits}`;
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
