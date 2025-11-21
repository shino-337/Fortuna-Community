import React, { useState, useEffect } from 'react';
import { ShieldCheck, Menu, X } from 'lucide-react';

const Navbar: React.FC = () => {
  const [isScrolled, setIsScrolled] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 20);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  return (
    <nav className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${isScrolled ? 'bg-black/90 backdrop-blur-md border-b border-gray-800' : 'bg-transparent'}`}>
      <div className="max-w-7xl mx-auto px-6 md:px-12 py-4 flex justify-between items-center">
        
        {/* Logo Text */}
        <div className="flex items-center gap-2 font-bold text-xl tracking-wider uppercase">
          <ShieldCheck className="w-8 h-8 text-pink-600" />
          <span>K8s<span className="text-pink-600">Fortuna</span></span>
        </div>

        {/* Desktop Menu */}
        <div className="hidden md:flex items-center gap-8 font-mono text-sm text-gray-400">
          <a href="#" className="hover:text-white transition-colors">Platform</a>
          <a href="#" className="hover:text-white transition-colors">Solutions</a>
          <a href="#" className="hover:text-white transition-colors">Pricing</a>
          <a href="#" className="hover:text-white transition-colors">Docs</a>
          <button className="px-5 py-2 border border-pink-600 text-pink-600 hover:bg-pink-600 hover:text-white transition-all uppercase text-xs font-bold tracking-widest">
            Login
          </button>
        </div>

        {/* Mobile Toggle */}
        <button className="md:hidden text-white" onClick={() => setMobileMenuOpen(!mobileMenuOpen)}>
          {mobileMenuOpen ? <X /> : <Menu />}
        </button>
      </div>

      {/* Mobile Menu */}
      {mobileMenuOpen && (
        <div className="md:hidden bg-black border-b border-gray-800 px-6 py-8 flex flex-col gap-4">
          <a href="#" className="text-gray-300 hover:text-pink-500 font-bold text-lg uppercase">Platform</a>
          <a href="#" className="text-gray-300 hover:text-pink-500 font-bold text-lg uppercase">Solutions</a>
          <a href="#" className="text-gray-300 hover:text-pink-500 font-bold text-lg uppercase">Pricing</a>
          <a href="#" className="text-pink-600 font-bold text-lg uppercase">Access Console</a>
        </div>
      )}
    </nav>
  );
};

export default Navbar;