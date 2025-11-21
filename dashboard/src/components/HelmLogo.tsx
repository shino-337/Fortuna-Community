import React from 'react'

interface HelmLogoProps {
  className?: string
  size?: number
}

const HelmLogo: React.FC<HelmLogoProps> = ({ className = "", size = 100 }) => {
  return (
    <div className={`relative flex items-center justify-center ${className}`} style={{ width: size, height: size }}>
      {/* Glow effect */}
      <div className="absolute inset-0 bg-pink-600 blur-3xl opacity-20 rounded-full pointer-events-none"></div>
      
      <svg
        width="100%"
        height="100%"
        viewBox="0 0 200 200"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        className="relative z-10 drop-shadow-2xl"
      >
        <defs>
          <linearGradient id="helmGradient" x1="100" y1="0" x2="100" y2="200" gradientUnits="userSpaceOnUse">
            <stop offset="0%" stopColor="#db2777" />
            <stop offset="100%" stopColor="#831843" />
          </linearGradient>
        </defs>

        {/* Decorative Tech Rings */}
        <circle cx="100" cy="100" r="85" stroke="#be185d" strokeWidth="1" strokeDasharray="4 4" opacity="0.6" />
        <circle cx="100" cy="100" r="75" stroke="#be185d" strokeWidth="1" opacity="0.3" />

        {/* Spokes */}
        <g stroke="url(#helmGradient)" strokeWidth="14" strokeLinecap="round">
          <line x1="100" y1="20" x2="100" y2="180" />
          <line x1="20" y1="100" x2="180" y2="100" />
          <line x1="43" y1="43" x2="157" y2="157" />
          <line x1="157" y1="43" x2="43" y2="157" />
        </g>

        {/* Rim */}
        <circle cx="100" cy="100" r="52" stroke="#db2777" strokeWidth="12" fill="none" />
        <circle cx="100" cy="100" r="52" stroke="black" strokeWidth="2" strokeOpacity="0.2" fill="none" />

        {/* Hub */}
        <circle cx="100" cy="100" r="24" fill="url(#helmGradient)" stroke="#be185d" strokeWidth="2" />

        {/* Keyhole */}
        <path
          d="M100 92 C104 92 107 95 107 99 C107 101 106 103 104 104 L 106 110 H 94 L 96 104 C 94 103 93 101 93 99 C93 95 96 92 100 92 Z"
          fill="#1a050b" 
        />
        
        {/* Highlight */}
        <circle cx="100" cy="100" r="52" stroke="white" strokeWidth="1" strokeOpacity="0.15" pointerEvents="none" />
      </svg>
    </div>
  )
}

export default HelmLogo

