import React, { useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion'; 
import { BarChart, Settings, ExternalLink, Plus, Trash2, Zap, Clock, Database, Pin, PinOff, AlertTriangle } from 'lucide-react';
import { EventsOn, BrowserOpenURL } from '../wailsjs/runtime/runtime';
import { UpdateConfig } from '../wailsjs/go/app/App';

const TierInput = ({ value, onChange }) => {
  const [displayValue, setDisplayValue] = useState((value || 0).toString());
  useEffect(() => { setDisplayValue((value || 0).toString()); }, [value]);
  const handleChange = (e) => { if (e.target.value.length > 5) return; setDisplayValue(e.target.value); };
  const handleBlur = () => {
    const sanitized = displayValue.replace(',', '.');
    let finalValue = parseFloat(sanitized);
    if (isNaN(finalValue) || finalValue < 0) finalValue = 0;
    onChange(finalValue);
  };
  return ( <input type="text" value={displayValue} onChange={handleChange} onBlur={handleBlur} className="w-full bg-black/50 border border-white/10 rounded-sm p-1 text-xs text-blue-400 font-bold" /> );
};

const SignalCard = ({ signal }) => {
  const isReturned = signal.status === 'RETURNED';
  const isTimeout = signal.status === 'TIMEOUT';
  const getWinRateColor = (prob) => {
  const val = parseFloat(prob);
  if (val <= 0) return 'text-slate-700';
  if (val > 60) return 'text-green-500';
  if (val > 30) return 'text-yellow-500';
  return 'text-red-500';
};


  return (
    <motion.div
      layout
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ 
        opacity: 1, scale: 1,
        borderColor: signal.isProgression && !isReturned && !isTimeout ? "rgba(59, 130, 246, 0.8)" : "rgba(255, 255, 255, 0.05)",
        boxShadow: signal.isProgression && !isReturned && !isTimeout ? "0 0 20px rgba(59, 130, 246, 0.2)" : "none"
      }}
      exit={{ opacity: 0, scale: 0.95, transition: { duration: 0.2 } }}
      transition={{ type: "spring", stiffness: 350, damping: 30 }}
      className={`relative p-4 rounded-sm border flex flex-col gap-3 transition-colors duration-500 ${
        signal.isPinned && !isReturned && !isTimeout ? 'bg-green-500/5 border-green-500/40 shadow-lg shadow-green-900/50' : 'bg-white/[0.02] border-white/5'
      }`}
    >
      {((signal.isPinned && !isReturned && !isTimeout) || isReturned || isTimeout) && (
        <div key={isReturned || isTimeout ? 'del' : signal.level} className={`absolute bottom-0 left-0 h-0.5 animate-shrink-width ${isReturned ? 'bg-green-900' : isTimeout ? 'bg-red-900' : 'bg-green-400'}`}></div>
      )}

      <div className="flex justify-between items-start">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <h3 className="text-lg font-black text-white italic tracking-tighter">{signal.symbol}</h3>
            <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded border ${signal.direction === 'UP' ? 'text-blue-400 border-blue-500/30' : 'text-red-400 border-red-500/30'}`}>
              {signal.direction} {signal.level}%
            </span>
            {signal.isProgression && !isReturned && !isTimeout && (
               <span className="text-[10px] bg-blue-600 text-white px-1 rounded animate-pulse uppercase font-black tracking-tighter">Progression</span>
            )}
          </div>
          <div className="text-[12px] text-slate-400 flex gap-3 font-bold uppercase font-mono italic opacity-60">
             <span className="flex items-center gap-1"><Database size={12}/> VOL24: {signal.volume ? (signal.volume / 1000000).toFixed(1) : "0"}M</span>
             <span className="flex items-center gap-1 opacity-40"><Clock size={12}/> {signal.timestamp}</span>
             <span className="text-slate-400 border-l border-white/10 pl-2">Time Window: {signal.activeWindow}m</span>
          </div>
        </div>
        <div className="text-right">
          <div className="text-[10px] text-slate-400 font-black uppercase tracking-widest mb-1">Win Probability</div>
          <div className={`text-xl font-black leading-none ${signal.prob > 0 ? getWinRateColor(signal.prob) : 'text-slate-700'}`}>
            {signal.prob > 0 ? `${signal.prob}%` : 'NaN'}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 bg-black/40 p-2.5 rounded border border-white/5 text-[10px] font-mono">
        <div className="space-y-1 border-r border-white/5 pr-2">
          <div className="text-[12px] text-slate-600 uppercase font-black mb-1 opacity-50 tracking-tighter">Reference Prices</div>
          <div className="flex justify-between"><span className="text-[12px]">Last Price:</span> <span className="text-[12px] text-slate-300">{signal.refLast}</span></div>
          <div className="flex justify-between"><span className="text-[12px]">Fair Price:</span> <span className="text-[12px] text-slate-300">{signal.refFair}</span></div>
        </div>
        <div className="space-y-1 pl-2">
          <div className="text-[12px] text-slate-600 uppercase font-black mb-1 opacity-50 tracking-tighter">Actual Prices</div>
          <div className=" flex justify-between"><span className="text-[12px]">Last Price:</span> <span className="text-[12px] text-blue-400 font-bold">{signal.lastPrice}</span></div>
          <div className="flex justify-between"><span className="text-[12px]">Fair Price:</span> <span className="text-[12px] text-blue-300">{signal.fairPrice}</span></div>
        </div>
      </div>

      <div className="flex justify-between items-center">
         <div className="flex gap-4 text-[10px] font-mono font-bold">
            <div className="flex flex-col"><span className="text-[8px] text-slate-600 uppercase font-bold">Basis Gap</span><span className="text-purple-400 font-bold">+{signal.gap}%</span></div>
            <div className="flex flex-col"><span className="text-[8px] text-slate-600 uppercase font-bold">Speed</span><span className="text-yellow-500 font-bold">{signal.speed}s</span></div>
         </div>
         <button onClick={() => BrowserOpenURL(`https://www.mexc.com/ru-RU/futures/${signal.symbol}?type=futures`)}
           className="px-4 py-1.5 rounded bg-blue-600/10 text-blue-500 hover:bg-blue-600 hover:text-white transition-all border border-blue-500/20 text-[9px] font-black uppercase flex items-center gap-2">
           Terminal <ExternalLink size={10} />
         </button>
      </div>

      <AnimatePresence>
        {(isReturned || isTimeout) && (
          <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: "auto", opacity: 1 }} 
                      className={`overflow-hidden border-t border-white/10 pt-2 flex justify-between items-center px-1 ${isTimeout ? 'bg-red-500/10' : 'bg-green-500/10'}`}>
             <div className={`text-[10px] font-black uppercase italic flex items-center gap-2 ${isTimeout ? 'text-red-400' : 'text-green-400'}`}>
                {isTimeout ? <AlertTriangle size={12}/> : <Zap size={12}/>}
                {isTimeout ? 'Window Exceeded' : `Price Returned | RT: ${signal.returnTime}s`}
             </div>
             <div className="text-[8px] text-slate-500 font-bold uppercase">Archive in 10s</div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
};

const App = () => {
  const [signals, setSignals] = useState([]);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [splashConfigs, setSplashConfigs] = useState([ 
    { level: 3, window: 10, isForcedPin: false }, 
    { level: 5, window: 15, isForcedPin: false } 
  ]);

  const configsRef = useRef(splashConfigs);
  useEffect(() => { configsRef.current = splashConfigs; }, [splashConfigs]);

  const getApplicableTier = (lvl) => {
    const configs = configsRef.current;
    if (!configs || configs.length === 0) return null;
    const sorted = [...configs].sort((a, b) => a.level - b.level);
    return sorted.filter(c => parseFloat(lvl) >= c.level).pop() || null;
  };

  useEffect(() => {
    const timer = setInterval(() => {
      const now = Date.now();
      setSignals(prev => {
        let hasChanges = false;
        const next = prev.map(s => {
          if (s.isPinned && s.unpinAt && now > s.unpinAt) {
            hasChanges = true;
            return { ...s, isPinned: false, unpinAt: null };
          }
          if (s.status === 'ACTIVE') {
            const tier = getApplicableTier(s.level);
            const limit = (tier?.window || s.activeWindow || 5) * 60 * 1000;
            if (now - s.createdAt > limit) {
              hasChanges = true;
              return { ...s, status: 'TIMEOUT', deleteAt: now + 10000, isPinned: false, unpinAt: null };
            }
          }
          return s;
        }).filter(s => {
          if (s.deleteAt && now > s.deleteAt) {
            hasChanges = true;
            return false;
          }
          return true;
        });
        return hasChanges ? next : prev;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  const handleNewSignal = (data) => {
    setSignals(prev => {
      const existingIdx = prev.findIndex(s => s.symbol === data.symbol);
      const now = Date.now();

      if (data.status === 'RETURNED' || data.status === 'TIMEOUT') {
        if (existingIdx === -1) return prev;
        const existing = prev[existingIdx];
        const updated = { ...existing, ...data, isPinned: false, deleteAt: now + 10000, unpinAt: null };
        return [...prev.filter((_, i) => i !== existingIdx), updated];
      }

      const currentTier = getApplicableTier(data.level);
      if (!currentTier) return prev; 

      if (existingIdx !== -1) {
        const existing = prev[existingIdx];
        if (existing.status !== 'ACTIVE' || data.direction !== existing.direction) return prev;

        const existingTier = getApplicableTier(existing.level);
       if (currentTier && existingTier && currentTier.level > existingTier.level) {
          const updated = {
            ...existing,
            ...data,
            isProgression: true,
            activeWindow: currentTier.window, 
            createdAt: Date.now(),          
            isPinned: (data.prob > 60 || currentTier.isForcedPin),
            unpinAt: (data.prob > 60 || currentTier.isForcedPin) ? now + 10000 : null,
          };
          const filtered = prev.filter((_, i) => i !== existingIdx);
          return [updated, ...filtered];
        }
      }

      const newSig = { ...data, id: `sig-${data.symbol}-${now}`, isPinned: (data.prob > 60 || currentTier.isForcedPin), unpinAt: (data.prob > 60 || currentTier.isForcedPin) ? now + 10000 : null, createdAt: now, activeWindow: currentTier.window, status: 'ACTIVE' };
      return [newSig, ...prev].slice(0, 50);
    });
  };

  useEffect(() => {
    const unsubscribe = EventsOn("splash:new", (data) => handleNewSignal(data));
    UpdateConfig({ tiers: splashConfigs });
    return () => unsubscribe();
  }, []);

  const saveStrategy = () => {
    UpdateConfig({ tiers: splashConfigs });
    setIsSettingsOpen(false);
  };

  const displaySignals = [...signals].sort((a, b) => {
    const aEnd = a.status !== 'ACTIVE';
    const bEnd = b.status !== 'ACTIVE';
    if (aEnd !== bEnd) return aEnd ? 1 : -1;
    if (a.isPinned !== b.isPinned) return a.isPinned ? -1 : 1;
    return b.level - a.level;
  });

  return (
    <div className="h-screen w-full bg-[#161515] text-[#e0e0e0] font-mono flex flex-col overflow-hidden">
      <header className="h-14 border-b border-white/5 bg-[#080808] flex items-center justify-between px-6 z-20 shrink-0">
        <div className="flex items-center gap-2 text-slate-100 font-black italic tracking-tighter text-xl"><BarChart size={20} /> Terminus</div>
        <button onClick={() => setIsSettingsOpen(!isSettingsOpen)} className="hover:bg-white/10 px-4 py-2 border border-white/10 text-[10px] font-black uppercase flex items-center gap-2"><Settings size={14}/> Config</button>
      </header>

      <main className="flex-1 flex overflow-hidden relative">
        <motion.div layout transition={{ duration: 0.4 }} className="flex-1 overflow-y-auto p-4 custom-scrollbar space-y-3">
          <AnimatePresence mode="popLayout">
            {displaySignals.map((sig) => ( <SignalCard key={sig.id} signal={sig} /> ))}
          </AnimatePresence>
        </motion.div>

        <AnimatePresence>
          {isSettingsOpen && (
            <motion.aside initial={{ x: "100%", width: 0 }} animate={{ x: 0, width: 380 }} exit={{ x: "100%", width: 0 }} 
               transition={{ type: 'spring', damping: 30, stiffness: 300 }}
               className="border-l border-white/5 bg-[#080808] p-6 flex flex-col gap-6 z-30 shrink-0 overflow-hidden"
            >
              <h2 className="text-[10px] font-black uppercase text-blue-500 italic tracking-widest">Splash Strategy</h2>
              <section className="space-y-4">
                {splashConfigs.map((cfg, idx) => (
                  <div key={idx} className="bg-white/5 p-3 border border-white/5 rounded-sm relative group">
                    <div className="grid grid-cols-[1fr_1fr_40px] gap-4 items-end">
                        <div><label className="text-[10px] text-slate-500 uppercase font-black">Level (%)</label>
                             <TierInput value={cfg.level} onChange={(v) => { const n = [...splashConfigs]; n[idx].level = v; setSplashConfigs(n); }} /></div>
                        <div><label className="text-[10px] text-slate-500 uppercase font-black">Window (m)</label>
                             <TierInput value={cfg.window} onChange={(v) => { const n = [...splashConfigs]; n[idx].window = v; setSplashConfigs(n); }} /></div>
                        <button onClick={() => { const n = [...splashConfigs]; n[idx].isForcedPin = !n[idx].isForcedPin; setSplashConfigs(n); }}
                          className={`h-7 w-full flex items-center justify-center rounded border transition-all ${cfg.isForcedPin ? 'bg-blue-600/20 border-blue-500 text-blue-400 shadow-md' : 'bg-black/40 border-white/10 text-slate-600'}`}>{cfg.isForcedPin ? <Pin size={14} /> : <PinOff size={14} />}</button>
                    </div>
                    <button onClick={() => setSplashConfigs(splashConfigs.filter((_, i) => i !== idx))} className="absolute -right-2 -top-2 bg-red-900/80 p-1 rounded-full text-white opacity-0 group-hover:opacity-100 transition-all"><Trash2 size={10}/></button>
                  </div>
                ))}
                <button onClick={() => setSplashConfigs([...splashConfigs, { level: 5, window: 5, isForcedPin: false }])} className="w-full py-2 border border-dashed border-white/10 text-slate-500 text-[9px] uppercase font-black hover:bg-white/5 transition-all tracking-widest">+ New Tier</button>
              </section>
              <button className="w-full bg-blue-600 py-3 text-[10px] font-black uppercase mt-auto tracking-widest" onClick={saveStrategy}>Save & Apply</button>
            </motion.aside>
          )}
        </AnimatePresence>
      </main>

      <footer className="h-6 border-t border-white/5 bg-[#050505] flex items-center px-6 justify-between text-[8px] font-bold text-slate-600 uppercase tracking-widest shrink-0">
        <span>Terminus Alpha v0.1</span>
        <span className="text-green-500 flex items-center gap-1"><Zap size={8}/> Parser Active</span>
      </footer>
    </div>
  );
};

export default App;
