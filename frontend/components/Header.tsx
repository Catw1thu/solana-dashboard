"use client";

import Link from "next/link";
import { Activity, Sun, Moon } from "lucide-react";
import { useTheme } from "../context/ThemeContext";

export const Header = () => {
  const { theme, toggleTheme } = useTheme();

  return (
    <header className="sticky top-0 z-50 border-b border-(--border-primary) bg-(--bg-primary)/80 backdrop-blur-xl">
      <div className="w-full flex h-14 items-center justify-between px-6">
        {/* Left Side: Logo & Title */}
        <Link
          href="/"
          className="flex items-center gap-3 hover:opacity-80 transition-opacity"
        >
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-(--accent-green)">
            <Activity className="h-4 w-4 text-black" />
          </div>
          <h1 className="text-lg font-bold tracking-tight">Solana Dashboard</h1>
        </Link>

        {/* Right Side */}
        <div className="flex items-center gap-4">
          {/* System Status */}
          <div className="flex items-center gap-2 text-xs font-medium text-(--accent-green)">
            <span className="live-dot" />
            <span className="hidden sm:inline">System Operational</span>
          </div>

          {/* Theme Toggle */}
          <button
            onClick={toggleTheme}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-(--border-primary) bg-(--bg-secondary) hover:bg-(--bg-tertiary) transition-colors"
            title={
              theme === "dark" ? "Switch to Light Mode" : "Switch to Dark Mode"
            }
          >
            {theme === "dark" ? (
              <Sun className="h-4 w-4 text-(--text-muted)" />
            ) : (
              <Moon className="h-4 w-4 text-(--text-muted)" />
            )}
          </button>
        </div>
      </div>
    </header>
  );
};
