import React from 'react'
import { Link } from 'react-router-dom'
import { Eye, Lock, Activity, Server, ShieldCheck, ShieldAlert, ChevronRight, Menu, X, Github, Twitter, Linkedin } from 'lucide-react'

/**
 * Landing Page for K8s Workload Management Platform
 * Professional, modern design with dark theme
 */
const LandingPage: React.FC = () => {
  const [mobileMenuOpen, setMobileMenuOpen] = React.useState(false)
  const [isScrolled, setIsScrolled] = React.useState(false)

  React.useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 20)
    }
    window.addEventListener('scroll', handleScroll)
    return () => window.removeEventListener('scroll', handleScroll)
  }, [])

  return (
    <div className="bg-black text-white selection:bg-pink-600 selection:text-white overflow-x-hidden font-sans">
      {/* Navbar */}
      <nav className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${isScrolled ? 'bg-black/90 backdrop-blur-md border-b border-gray-800' : 'bg-transparent'}`}>
        <div className="max-w-7xl mx-auto px-6 md:px-12 py-4 flex justify-between items-center">
          
          {/* Logo */}
          <div className="flex items-center gap-2 font-bold text-xl tracking-wider uppercase">
            <ShieldCheck className="w-8 h-8 text-pink-600" />
            <span>K8s<span className="text-pink-600">Fortuna</span></span>
          </div>

          {/* Desktop Menu */}
          <div className="hidden md:flex items-center gap-8 font-mono text-sm text-gray-400">
            <a href="#features" className="hover:text-white transition-colors">Platform</a>
            <a href="#features" className="hover:text-white transition-colors">Features</a>
            <a href="https://github.com" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Docs</a>
            <Link 
              to="/login"
              className="px-5 py-2 border border-pink-600 text-pink-600 hover:bg-pink-600 hover:text-white transition-all uppercase text-xs font-bold tracking-widest"
            >
              Login
            </Link>
          </div>

          {/* Mobile Toggle */}
          <button className="md:hidden text-white" onClick={() => setMobileMenuOpen(!mobileMenuOpen)}>
            {mobileMenuOpen ? <X /> : <Menu />}
          </button>
        </div>

        {/* Mobile Menu */}
        {mobileMenuOpen && (
          <div className="md:hidden bg-black border-b border-gray-800 px-6 py-8 flex flex-col gap-4">
            <a href="#features" className="text-gray-300 hover:text-pink-500 font-bold text-lg uppercase">Platform</a>
            <a href="#features" className="text-gray-300 hover:text-pink-500 font-bold text-lg uppercase">Features</a>
            <a href="https://github.com" target="_blank" rel="noopener noreferrer" className="text-gray-300 hover:text-pink-500 font-bold text-lg uppercase">Docs</a>
            <Link to="/login" className="text-pink-600 font-bold text-lg uppercase">Login</Link>
          </div>
        )}
      </nav>

      {/* Hero Section */}
      <section className="relative w-full min-h-[85vh] flex flex-col md:flex-row items-center justify-center px-6 md:px-16 lg:px-32 py-20 overflow-hidden bg-black">
        
        {/* Background Grid */}
        <div className="absolute inset-0 z-0 opacity-10" 
             style={{ backgroundImage: 'radial-gradient(#333 1px, transparent 1px)', backgroundSize: '30px 30px' }}>
        </div>

        <div className="z-10 flex flex-col md:flex-row items-center w-full max-w-7xl gap-12 md:gap-16">
          
          {/* Logo */}
          <div className="flex-shrink-0 animate-fade-in-up">
            <HelmLogo size={320} className="md:w-[400px] md:h-[400px]" />
          </div>

          {/* Content */}
          <div className="flex flex-col text-center md:text-left space-y-6 max-w-2xl">
            
            {/* Title */}
            <h1 className="text-4xl md:text-6xl lg:text-7xl font-black tracking-tight text-white uppercase leading-tight">
              K8s Fortuna <br/>
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-white to-gray-400">
                Workload
              </span> <br/>
              Management
            </h1>

            {/* Subtitle */}
            <div className="border-l-4 border-pink-600 pl-6 py-2">
              <h2 className="text-lg md:text-xl lg:text-2xl font-bold tracking-[0.2em] text-pink-600 uppercase">
                Visibility First. <span className="text-white">Security Always.</span>
              </h2>
            </div>

            {/* CTA Buttons */}
            <div className="pt-8 flex flex-col sm:flex-row gap-4 justify-center md:justify-start">
              <Link 
                to="/login"
                className="group relative px-8 py-4 bg-pink-700 hover:bg-pink-600 text-white font-bold uppercase tracking-wider transition-all duration-200 ease-in-out overflow-hidden rounded-sm text-center"
              >
                <span className="relative z-10 flex items-center justify-center gap-2">
                  Initialize System <ChevronRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
                </span>
                <div className="absolute inset-0 -translate-x-full group-hover:translate-x-0 bg-pink-500 transition-transform duration-300 ease-out skew-x-12 origin-left"></div>
              </Link>
              
              <a 
                href="https://github.com" 
                target="_blank" 
                rel="noopener noreferrer"
                className="px-8 py-4 border border-gray-700 hover:border-white text-gray-300 hover:text-white font-bold uppercase tracking-wider transition-colors duration-200 rounded-sm text-center flex items-center justify-center gap-2"
              >
                View Documentation
              </a>
            </div>

          </div>
        </div>
      </section>

      {/* Features Section */}
      <section id="features" className="bg-black py-24 px-6 md:px-12 border-t border-gray-900">
        <div className="max-w-7xl mx-auto">
          <div className="mb-16 text-center md:text-left">
            <h3 className="text-pink-600 font-mono text-sm uppercase tracking-widest mb-2">Architecture</h3>
            <h2 className="text-3xl md:text-5xl font-bold text-white uppercase max-w-2xl">
              Built for the <span className="text-gray-500">Hostile</span> Network
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {features.map((feature, index) => (
              <div key={index} className="group p-8 bg-gray-950 border border-gray-900 hover:border-pink-900 transition-colors duration-300 rounded-sm relative overflow-hidden">
                <div className="absolute top-0 left-0 w-1 h-0 group-hover:h-full bg-pink-600 transition-all duration-300 ease-out"></div>
                
                <feature.icon className="w-10 h-10 text-pink-700 mb-6 group-hover:text-pink-500 transition-colors" />
                
                <h4 className="text-xl font-bold text-white uppercase mb-3 tracking-wide">
                  {feature.title}
                </h4>
                <p className="text-gray-400 text-sm leading-relaxed">
                  {feature.description}
                </p>
              </div>
            ))}
          </div>

          {/* CTA Banner */}
          <div className="mt-24 p-8 md:p-12 bg-gradient-to-r from-gray-900 to-black border border-gray-800 rounded-lg flex flex-col md:flex-row items-center justify-between gap-8">
            <div className="flex items-center gap-6">
              <div className="bg-pink-900/20 p-4 rounded-full border border-pink-900/50">
                <ShieldAlert className="w-12 h-12 text-pink-500" />
              </div>
              <div>
                <h4 className="text-2xl font-bold text-white">Secure Your Workload Now</h4>
                <p className="text-gray-400">Join 500+ enterprises securing their Kubernetes environment.</p>
              </div>
            </div>
            <Link 
              to="/login"
              className="w-full md:w-auto px-8 py-4 bg-white text-black font-black uppercase hover:bg-gray-200 transition-colors text-center"
            >
              Get a Demo
            </Link>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="bg-black border-t border-gray-900 py-12 px-6">
        <div className="max-w-7xl mx-auto">
          <div className="flex flex-col md:flex-row justify-between items-center gap-6 mb-8">
            <div className="text-gray-500 text-sm font-mono">
              &copy; {new Date().getFullYear()} K8S WORKLOAD MANAGEMENT. ALL RIGHTS RESERVED.
            </div>
            <div className="flex gap-6 text-sm font-bold uppercase tracking-wider text-gray-600">
              <a href="https://github.com" target="_blank" rel="noopener noreferrer" className="hover:text-pink-600 transition-colors">
                <Github className="w-5 h-5" />
              </a>
              <a href="#" className="hover:text-pink-600 transition-colors">
                <Twitter className="w-5 h-5" />
              </a>
              <a href="#" className="hover:text-pink-600 transition-colors">
                <Linkedin className="w-5 h-5" />
              </a>
            </div>
          </div>
          <div className="flex flex-col md:flex-row justify-between items-center gap-4 text-xs text-gray-600">
            <div className="flex gap-6">
              <a href="#" className="hover:text-pink-600 transition-colors">Privacy Policy</a>
              <a href="#" className="hover:text-pink-600 transition-colors">Terms of Service</a>
              <a href="#" className="hover:text-pink-600 transition-colors">Contact</a>
            </div>
            <div>
              Built with ❤️ for Kubernetes
            </div>
          </div>
        </div>
      </footer>
    </div>
  )
}

// Features data
const features = [
  {
    icon: Eye,
    title: "Deep Visibility",
    description: "Real-time introspection into every pod, service, and node. See what's happening before it becomes an incident."
  },
  {
    icon: Lock,
    title: "Zero Trust Security",
    description: "Enforce strict network policies and mTLS automatically. Default deny, explicit allow architecture."
  },
  {
    icon: Activity,
    title: "Live Threat Detection",
    description: "AI-driven anomaly detection for runtime behaviors. Catch crypto-mining and shell executions instantly."
  },
  {
    icon: Server,
    title: "Multi-Cluster Control",
    description: "Manage security posture across hybrid cloud and on-premise K8s clusters from a single pane of glass."
  }
]

// Helm Logo Component
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

export default LandingPage

