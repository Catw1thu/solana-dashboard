import type { Metadata } from "next";
import { Inter, Manrope } from "next/font/google";
import "./globals.css";
import { SocketProvider } from "../context/SocketContext";
import { ThemeProvider } from "../context/ThemeContext";
import { Header } from "../components/Header";

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
        <ThemeProvider>
          <SocketProvider>
            <div className="min-h-screen bg-(--bg-primary) text-(--text-primary) font-sans selection:bg-(--accent-green)/30">
              <Header />
              {children}
            </div>
          </SocketProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
