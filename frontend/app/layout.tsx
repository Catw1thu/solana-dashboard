import type { Metadata } from "next";
import { Inter, Manrope } from "next/font/google"; // Premium fonts
import "./globals.css";
import { SocketProvider } from "../context/SocketContext";

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
});

const manrope = Manrope({
  variable: "--font-manrope",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Solana Dashboard",
  description: "Real-time token analytics",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${inter.variable} ${manrope.variable} antialiased`}>
        <SocketProvider>{children}</SocketProvider>
      </body>
    </html>
  );
}
