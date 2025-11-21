import React from 'react';
import { Eye, Lock, Activity, Server, Zap, ShieldAlert } from 'lucide-react';

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
];

const Features: React.FC = () => {
  return (
    <section className="bg-black py-24 px-6 md:px-12 border-t border-gray-900">
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

        {/* Stat Banner */}
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
          <button className="w-full md:w-auto px-8 py-4 bg-white text-black font-black uppercase hover:bg-gray-200 transition-colors">
            Get a Demo
          </button>
        </div>
      </div>
    </section>
  );
};

export default Features;