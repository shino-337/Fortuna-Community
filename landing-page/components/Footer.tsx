import React from 'react';

const Footer: React.FC = () => {
  return (
    <footer className="bg-black border-t border-gray-900 py-12 px-6 text-center md:text-left">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row justify-between items-center gap-6">
        <div className="text-gray-500 text-sm font-mono">
          &copy; {new Date().getFullYear()} K8S FORTUNA. ALL RIGHTS RESERVED.
        </div>
        <div className="flex gap-6 text-sm font-bold uppercase tracking-wider text-gray-600">
            <a href="#" className="hover:text-pink-600 transition-colors">Privacy</a>
            <a href="#" className="hover:text-pink-600 transition-colors">Terms</a>
            <a href="#" className="hover:text-pink-600 transition-colors">Contact</a>
        </div>
      </div>
    </footer>
  );
};

export default Footer;