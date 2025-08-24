"use client";
import { useEffect, useState } from "react";

export default function Brand({ width = 96 }: { width?: number }) {
  const [theme, setTheme] = useState<"light" | "dark">();

  useEffect(() => {
    const root = document.documentElement;
    const read = () => (root.dataset.theme as "light" | "dark" | undefined) || (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    setTheme(read());
    const mo = new MutationObserver(() => setTheme(read()));
    mo.observe(root, { attributes: true, attributeFilter: ["data-theme"] });
    return () => mo.disconnect();
  }, []);

  const isDark = theme === "dark";
  return (
    <span className="inline-flex items-center">
      <img src="/lamdis_white.webp" width={width} alt="Lamdis" style={{ display: isDark ? "inline-block" : "none" }} />
      <img src="/lamdis_black.webp" width={width} alt="Lamdis" style={{ display: !isDark ? "inline-block" : "none" }} />
    </span>
  );
}
