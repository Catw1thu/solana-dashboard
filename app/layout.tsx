import type { Metadata } from "next";
import { Inter, Manrope } from "next/font/google";
import { cookies } from "next/headers";
import "./globals.css";
import { ThemeProvider } from "../context/ThemeContext";
import { WebSocketProvider } from "../context/WebSocketContext";
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

const themeInitScript = `
(() => {
  try {
    const savedTheme = window.localStorage.getItem("theme");
    const attrTheme = document.documentElement.getAttribute("data-theme");
    const theme =
      savedTheme === "light" || savedTheme === "dark"
        ? savedTheme
        : attrTheme === "light" || attrTheme === "dark"
          ? attrTheme
          : "dark";
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.style.colorScheme = theme;
    window.localStorage.setItem("theme", theme);
  } catch {
    document.documentElement.setAttribute("data-theme", "dark");
    document.documentElement.style.colorScheme = "dark";
  }
})();
`;

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const cookieStore = await cookies();
  const themeCookie = cookieStore.get("theme")?.value;
  const initialTheme = themeCookie === "light" ? "light" : "dark";

  return (
    <html
      lang="en"
      data-theme={initialTheme}
      style={{ colorScheme: initialTheme }}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInitScript }} />
      </head>
      <body className={`${inter.variable} ${manrope.variable} antialiased`}>
        <ThemeProvider>
          <WebSocketProvider>
            <div className="min-h-screen bg-(--bg-primary) text-(--text-primary) font-sans selection:bg-(--accent-green)/30">
              <Header />
              {children}
            </div>
          </WebSocketProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
