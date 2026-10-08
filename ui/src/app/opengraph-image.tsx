import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { ImageResponse } from "next/og";
import { SITE } from "@/constants/site";

// Rendered once at build time and served as a static PNG.
export const alt = `${SITE.name}: compress video, audio and images. Built by ${SITE.author.name}.`;
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const BG = "#0a0a0a";
const CARD = "#171717";
const BORDER = "rgba(255,255,255,0.12)";
const FG = "#fafafa";
const MUTED = "#a1a1a1";
const GREEN = "#34d399";

const SAMPLE_JOBS = [
  { name: "holiday-4k.mov", kind: "VIDEO", from: "248 MB", to: "9.8 MB", saved: 96, note: "H.265 · 1080p" },
  { name: "podcast-ep12.wav", kind: "AUDIO", from: "112 MB", to: "14 MB", saved: 88, note: "Opus · -16 LUFS" },
  { name: "screenshot.png", kind: "IMAGE", from: "4.2 MB", to: "380 KB", saved: 91, note: "AVIF · EXIF stripped" },
];

export default async function OpengraphImage() {
  const logo = await readFile(join(process.cwd(), "public/images/favicon_io/android-chrome-192x192.png"));
  const logoSrc = `data:image/png;base64,${logo.toString("base64")}`;

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          padding: "64px 72px",
          background: BG,
          backgroundImage: "radial-gradient(circle at 85% 10%, rgba(255,255,255,0.08), transparent 45%)",
          color: FG,
          fontFamily: "monospace",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 56 }}>
          {/* Pitch */}
          <div style={{ display: "flex", flexDirection: "column", width: 500 }}>
            <div style={{ display: "flex", fontSize: 76, fontWeight: 700, letterSpacing: -3 }}>elcompresso</div>
            <div style={{ display: "flex", marginTop: 20, fontSize: 30, lineHeight: 1.35, color: MUTED }}>
              Compress, convert and trim video, audio and images. Hit an exact file size.
            </div>
            <div style={{ display: "flex", gap: 10, marginTop: 32 }}>
              {["Discord 10 MB", "WhatsApp 16 MB", "Email 25 MB"].map((t) => (
                <div
                  key={t}
                  style={{ display: "flex", padding: "8px 14px", border: `1px solid ${BORDER}`, fontSize: 20, color: FG }}
                >
                  {t}
                </div>
              ))}
            </div>
          </div>

          {/* Mock job list, like the app's results panel */}
          <div style={{ display: "flex", flexDirection: "column", width: 520, background: CARD, border: `1px solid ${BORDER}` }}>
            {SAMPLE_JOBS.map((j, i) => (
              <div
                key={j.name}
                style={{
                  display: "flex",
                  flexDirection: "column",
                  padding: "20px 24px",
                  borderTop: i ? `1px solid ${BORDER}` : "none",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                    <div style={{ display: "flex", fontSize: 14, color: MUTED, border: `1px solid ${BORDER}`, padding: "2px 6px" }}>
                      {j.kind}
                    </div>
                    <div style={{ display: "flex", fontSize: 22 }}>{j.name}</div>
                  </div>
                  <div
                    style={{
                      display: "flex",
                      fontSize: 20,
                      fontWeight: 700,
                      color: GREEN,
                      background: "rgba(52,211,153,0.15)",
                      padding: "2px 8px",
                    }}
                  >
                    -{j.saved}%
                  </div>
                </div>
                <div style={{ display: "flex", marginTop: 8, fontSize: 18, color: MUTED }}>
                  {j.from} → <span style={{ color: FG, marginLeft: 8, marginRight: 8 }}>{j.to}</span> · {j.note}
                </div>
                <div style={{ display: "flex", marginTop: 12, height: 6, background: "rgba(255,255,255,0.1)" }}>
                  <div style={{ display: "flex", width: `${100 - j.saved}%`, background: FG }} />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Byline */}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 18 }}>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={logoSrc} width={56} height={56} alt="" style={{ borderRadius: 999 }} />
            <div style={{ display: "flex", flexDirection: "column" }}>
              <div style={{ display: "flex", fontSize: 18, color: MUTED }}>Built by</div>
              <div style={{ display: "flex", fontSize: 28, fontWeight: 700 }}>{SITE.author.name}</div>
            </div>
          </div>
          <div style={{ display: "flex", fontSize: 24, color: MUTED }}>umohsg.com</div>
        </div>
      </div>
    ),
    size
  );
}
