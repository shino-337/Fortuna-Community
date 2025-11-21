import React from 'react';
import Logo from './Logo';
import { ChevronRight } from 'lucide-react';

const Hero: React.FC = () => {
  return (
    <section className="relative w-full min-h-[85vh] flex flex-col md:flex-row items-center justify-center px-6 md:px-16 lg:px-32 py-20 overflow-hidden bg-black">
      
      {/* Background Grid Texture */}
      <div className="absolute inset-0 z-0 opacity-10" 
           style={{ backgroundImage: 'radial-gradient(#333 1px, transparent 1px)', backgroundSize: '30px 30px' }}>
      </div>

      <div className="z-10 flex flex-col md:flex-row items-center w-full max-w-7xl gap-12 md:gap-16">
        
        {/* Left Side: The Logo (Prominent as in image) */}
        <div className="flex-shrink-0 animate-fade-in-up">
          <Logo size={320} className="md:w-[400px] md:h-[400px]" />
        </div>

        {/* Right Side: Typography */}
        <div className="flex flex-col text-center md:text-left space-y-6 max-w-2xl">
          
          {/* Main Headline */}
          <h1 className="text-4xl md:text-6xl lg:text-7xl font-black tracking-tight text-white uppercase leading-tight">
            K8s Workload <br/>
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-white to-gray-400">
              Management
            </span> <br/>
            Platform
          </h1>

          {/* Tagline / Subtitle - Matching the pink uppercase style */}
          <div className="border-l-4 border-pink-600 pl-6 py-2">
            <h2 className="text-lg md:text-xl lg:text-2xl font-bold tracking-[0.2em] text-pink-600 uppercase">
              Visibility First. <span className="text-white">Security Always.</span>
            </h2>
          </div>

          {/* CTA Buttons (Added for functionality) */}
          <div className="pt-8 flex flex-col sm:flex-row gap-4 justify-center md:justify-start">
            <button className="group relative px-8 py-4 bg-pink-700 hover:bg-pink-600 text-white font-bold uppercase tracking-wider transition-all duration-200 ease-in-out overflow-hidden rounded-sm">
              <span className="relative z-10 flex items-center gap-2">
                Initialize System <ChevronRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
              </span>
              <div className="absolute inset-0 -translate-x-full group-hover:translate-x-0 bg-pink-500 transition-transform duration-300 ease-out skew-x-12 origin-left"></div>
            </button>
            
            <button className="px-8 py-4 border border-gray-700 hover:border-white text-gray-300 hover:text-white font-bold uppercase tracking-wider transition-colors duration-200 rounded-sm">
              View Documentation
            </button>
          </div>

        </div>
      </div>
    </section>
  );
};

export default Hero;