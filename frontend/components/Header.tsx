"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Activity, Sun, Moon, Search, X } from "lucide-react";
import { useTheme } from "../context/ThemeContext";
import { useState, useEffect, useRef, useCallback } from "react";
import { API } from "../config/api";

interface SearchResult {
  mint: string;
  name: string | null;
  symbol: string | null;
  image: string | null;
  price: number | null;
}

export const Header = () => {
  const { theme, toggleTheme } = useTheme();
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Debounced search
  const doSearch = useCallback(async (q: string) => {
    if (q.length < 2) {
      setResults([]);
      setIsOpen(false);
      return;
    }
    setIsLoading(true);
    try {
      const res = await fetch(API.search(q));
      const data = await res.json();
      setResults(data);
      setIsOpen(data.length > 0);
      setSelectedIndex(-1);
    } catch {
      setResults([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const handleChange = (value: string) => {
    setQuery(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => doSearch(value), 300);
  };

  const handleSelect = (mint: string) => {
    setQuery("");
    setIsOpen(false);
    setResults([]);
    router.push(`/pair/${mint}`);
  };

  // Keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isOpen || results.length === 0) {
      if (e.key === "Escape") {
        setQuery("");
        setIsOpen(false);
        inputRef.current?.blur();
      }
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((prev) => Math.min(prev + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((prev) => Math.max(prev - 1, 0));
    } else if (e.key === "Enter" && selectedIndex >= 0) {
      e.preventDefault();
      handleSelect(results[selectedIndex].mint);
    } else if (e.key === "Escape") {
      setIsOpen(false);
      setQuery("");
      inputRef.current?.blur();
    }
  };

  // Click outside to close
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <header className="sticky top-0 z-50 border-b border-(--border-primary) bg-(--bg-primary)/80 backdrop-blur-xl">
      <div className="w-full flex h-14 items-center justify-between px-6 gap-4">
        {/* Left Side: Logo & Title */}
        <Link
          href="/"
          className="flex items-center gap-3 hover:opacity-80 transition-opacity shrink-0"
        >
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-(--accent-green)">
            <Activity className="h-4 w-4 text-black" />
          </div>
          <h1 className="text-lg font-bold tracking-tight hidden sm:block">
            Solana Dashboard
          </h1>
        </Link>

        {/* Center: Search */}
        <div ref={containerRef} className="relative flex-1 max-w-md">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-(--text-muted) pointer-events-none" />
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => handleChange(e.target.value)}
              onKeyDown={handleKeyDown}
              onFocus={() => {
                if (results.length > 0) setIsOpen(true);
              }}
              placeholder="Search token name or mint..."
              className="w-full h-9 pl-9 pr-8 rounded-lg border border-(--border-primary) bg-(--bg-secondary) text-sm text-(--text-primary) placeholder:text-(--text-muted) outline-none focus:border-(--accent-green)/50 focus:ring-1 focus:ring-(--accent-green)/20 transition-all"
            />
            {query && (
              <button
                onClick={() => {
                  setQuery("");
                  setResults([]);
                  setIsOpen(false);
                  inputRef.current?.focus();
                }}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 rounded hover:bg-(--bg-tertiary) text-(--text-muted)"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>

          {/* Dropdown */}
          {isOpen && (
            <div className="absolute top-full left-0 right-0 mt-1 rounded-lg border border-(--border-primary) bg-(--bg-secondary) shadow-xl overflow-hidden z-60">
              {isLoading ? (
                <div className="px-4 py-3 text-sm text-(--text-muted) text-center">
                  Searching...
                </div>
              ) : results.length === 0 ? (
                <div className="px-4 py-3 text-sm text-(--text-muted) text-center">
                  No results found
                </div>
              ) : (
                <div className="max-h-80 overflow-y-auto">
                  {results.map((r, i) => (
                    <button
                      key={r.mint}
                      onClick={() => handleSelect(r.mint)}
                      onMouseEnter={() => setSelectedIndex(i)}
                      className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors ${
                        i === selectedIndex
                          ? "bg-(--bg-tertiary)"
                          : "hover:bg-(--bg-tertiary)/50"
                      }`}
                    >
                      {/* Avatar */}
                      {r.image ? (
                        <img
                          src={r.image}
                          alt={r.symbol || ""}
                          className="w-8 h-8 rounded-full object-cover shrink-0 ring-1 ring-(--border-primary)"
                          onError={(e) => {
                            (e.target as HTMLImageElement).style.display =
                              "none";
                          }}
                        />
                      ) : (
                        <div className="w-8 h-8 rounded-full bg-(--bg-tertiary) shrink-0 ring-1 ring-(--border-primary) flex items-center justify-center text-xs font-bold text-(--text-muted)">
                          {r.symbol ? r.symbol[0] : "?"}
                        </div>
                      )}
                      {/* Info */}
                      <div className="flex flex-col min-w-0 flex-1">
                        <span className="text-sm font-semibold text-(--text-primary) truncate">
                          {r.symbol || "Unknown"}
                        </span>
                        <span className="text-xs text-(--text-muted) truncate">
                          {r.name ||
                            `${r.mint.slice(0, 8)}...${r.mint.slice(-4)}`}
                        </span>
                      </div>
                      {/* Mint */}
                      <span className="text-xs font-mono text-(--text-muted) shrink-0">
                        {r.mint.slice(0, 4)}...{r.mint.slice(-4)}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Right Side */}
        <div className="flex items-center gap-4 shrink-0">
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
