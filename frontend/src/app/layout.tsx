import type { Metadata } from "next";
import { JetBrains_Mono } from "next/font/google";
import "./globals.css";

const mono = JetBrains_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Mycelium",
  description: "Local code intelligence",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className={`${mono.variable} font-mono antialiased`}>
        <div className="min-h-screen">
          <header className="border-b border-border px-6 py-3 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <img src="/icon.svg" alt="" width={20} height={20} />
              <span className="text-sm font-medium tracking-tight">
                mycelium
              </span>
              <span className="text-xs text-muted-foreground">v0.1</span>
            </div>
          </header>
          <main>{children}</main>
        </div>
      </body>
    </html>
  );
}
