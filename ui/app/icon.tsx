import { ImageResponse } from "next/og"

export const size = { width: 32, height: 32 }
export const contentType = "image/png"

export default function Icon() {
  return new ImageResponse(
    (
      <div
        style={{
          background: "#18181b",
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          borderRadius: 7,
          position: "relative",
        }}
      >
        {/* Timeline line */}
        <div
          style={{
            position: "absolute",
            top: 15,
            left: 5,
            right: 5,
            height: 2,
            background: "#3f3f46",
            borderRadius: 1,
          }}
        />
        {/* Left dot */}
        <div
          style={{
            position: "absolute",
            left: 6,
            top: 13,
            width: 6,
            height: 6,
            background: "#71717a",
            borderRadius: "50%",
          }}
        />
        {/* Middle dot (active, blue) */}
        <div
          style={{
            position: "absolute",
            left: 12,
            top: 11,
            width: 9,
            height: 9,
            background: "#3b82f6",
            borderRadius: "50%",
          }}
        />
        {/* Right dot */}
        <div
          style={{
            position: "absolute",
            left: 20,
            top: 13,
            width: 6,
            height: 6,
            background: "#71717a",
            borderRadius: "50%",
          }}
        />
      </div>
    ),
    { ...size },
  )
}
