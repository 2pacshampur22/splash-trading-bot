import React, { useState, useEffect, useRef, useCallback } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { EventsOn, BrowserOpenURL } from '../wailsjs/runtime/runtime';
import { UpdateConfig, UpdateSpreadConfig, GetLatestSpreads } from '../wailsjs/go/app/App';

// ─── URLs ─────────────────────────────────────────────────────────────────────
const withUsdt = (s) => s.includes('_') ? s : `${s}_USDT`;
const base     = (s) => s.replace('_USDT','').replace('-USDT','');
const EXCHANGE_URLS = {
  MEXC:        (s) => `https://futures.mexc.com/exchange/${withUsdt(s)}`,
  OKX:         (s) => `https://www.okx.com/trade-swap/${base(s).toLowerCase()}-usdt-swap`,
  'Gate.io':   (s) => `https://www.gate.io/futures/USDT/${withUsdt(s)}`,
  BingX:       (s) => `https://bingx.com/en/perpetual/${base(s)}-USDT/`,
  Hyperliquid: (s) => `https://app.hyperliquid.xyz/trade/${base(s)}`,
};
const getUrl = (ex, sym) => (EXCHANGE_URLS[ex] || (() => '#'))(sym);

const EXCHANGES = ['MEXC','OKX','Gate.io','BingX','Hyperliquid'];
const LEVELS    = [1,2,3,5,7,10];

// ─── Colors ───────────────────────────────────────────────────────────────────
const C = {
  up:     { t:'#4ade80', bg:'rgba(74,222,128,0.08)',   b:'rgba(74,222,128,0.22)'  },
  down:   { t:'#f87171', bg:'rgba(248,113,113,0.08)',  b:'rgba(248,113,113,0.22)' },
  blue:   { t:'#60a5fa', bg:'rgba(96,165,250,0.1)',    b:'rgba(96,165,250,0.28)'  },
  orange: { t:'#fb923c', bg:'rgba(251,146,60,0.1)',    b:'rgba(251,146,60,0.28)'  },
  purple: { t:'#a78bfa', bg:'rgba(167,139,250,0.1)',   b:'rgba(167,139,250,0.28)' },
};
const exClr   = (ex) => ({ MEXC:C.blue, OKX:C.blue, 'Gate.io':C.purple, BingX:C.orange, Hyperliquid:C.purple }[ex] || C.blue);
const probClr = (p)  => p <= 0 ? '#3f3f46' : p >= 60 ? '#4ade80' : p >= 35 ? '#facc15' : '#f87171';
const sprdClr = (p)  => p >= 3 ? '#f87171' : p >= 1.5 ? '#fb923c' : p >= 0.5 ? '#facc15' : '#a3e635';

// ─── Icons ────────────────────────────────────────────────────────────────────
const Ico = ({ d, size=14, color='currentColor', sw=1.6 }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" style={{flexShrink:0}}>
    <path d={d}/>
  </svg>
);
const I = {
  settings: 'M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z',
  close:    'M18 6L6 18M6 6l12 12',
  link:     'M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3',
  plus:     'M12 5v14M5 12h14',
  trash:    'M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6',
  pin:      'M12 2l3 7h5l-4 3 2 7-6-4-6 4 2-7-4-3h5z',
  pinOff:   'M2 2l20 20M12 2l3 7h5l-4 3 2 7-6-4-3 2M9 5H7l-4 3h5',
  zap:      'M13 2L3 14h9l-1 8 10-12h-9l1-8z',
  arrows:   'M7 16V4m0 0L3 8m4-4l4 4M17 8v12m0 0l4-4m-4 4l-4-4',
  filter:   'M22 3H2l8 9.46V19l4 2v-8.54L22 3z',
  check:    'M20 6L9 17l-5-5',
};

// ─── Primitives ───────────────────────────────────────────────────────────────
const NumInput = ({ label, value, onChange, suffix }) => {
  const [local, setLocal] = useState(String(value));
  useEffect(() => setLocal(String(value)), [value]);
  return (
    <div style={{ display:'flex', flexDirection:'column', gap:4 }}>
      <label style={{ fontSize:10, color:'#52525b', textTransform:'uppercase', letterSpacing:'0.08em', fontWeight:600 }}>{label}</label>
      <div style={{ display:'flex', alignItems:'center', background:'#0d0d0f', border:'1px solid #27272a', borderRadius:6 }}>
        <input type="text" value={local}
          onChange={e => setLocal(e.target.value)}
          onBlur={() => { const v=parseFloat(local.replace(',','.')); if(!isNaN(v)) onChange(v); }}
          style={{ flex:1, background:'transparent', border:'none', outline:'none', color:'#e4e4e7', fontSize:13, padding:'7px 10px', fontFamily:'monospace', width:0 }}
        />
        {suffix && <span style={{ fontSize:11, color:'#3f3f46', paddingRight:10 }}>{suffix}</span>}
      </div>
    </div>
  );
};

const Toggle = ({ label, value, onChange }) => (
  <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', gap:12 }}>
    <span style={{ fontSize:12, color:'#71717a' }}>{label}</span>
    <div onClick={() => onChange(!value)} style={{ width:36, height:20, borderRadius:10, background:value?'#2563eb':'#27272a', cursor:'pointer', position:'relative', transition:'background 0.2s', flexShrink:0 }}>
      <motion.div animate={{ left: value ? 18 : 2 }} transition={{ type:'spring', stiffness:500, damping:30 }}
        style={{ position:'absolute', top:2, width:16, height:16, borderRadius:'50%', background:'#fff', boxShadow:'0 1px 3px rgba(0,0,0,0.4)' }} />
    </div>
  </div>
);

const Chip = ({ label, active, onClick, color }) => (
  <motion.button onClick={onClick} whileTap={{ scale:0.93 }} style={{
    padding:'5px 11px', fontSize:11, fontWeight:600, borderRadius:5, cursor:'pointer', border:'1px solid', userSelect:'none',
    background: active ? (color?.bg||'rgba(96,165,250,0.12)') : 'transparent',
    borderColor: active ? (color?.b||'rgba(96,165,250,0.4)') : '#27272a',
    color: active ? (color?.t||'#60a5fa') : '#52525b',
    transition:'color 0.12s, background 0.12s, border-color 0.12s',
  }}>{label}</motion.button>
);

const Badge = ({ label, c }) => (
  <span style={{ fontSize:10, padding:'2px 7px', borderRadius:4, background:c.bg, color:c.t, border:`1px solid ${c.b}`, fontWeight:600, whiteSpace:'nowrap' }}>
    {label}
  </span>
);

// ─── Splash Screen ────────────────────────────────────────────────────────────
const SplashScreen = ({ onDone }) => {
  const [phase, setPhase] = useState(0);
  useEffect(() => {
    const t1 = setTimeout(() => setPhase(1), 500);
    const t2 = setTimeout(() => setPhase(2), 1700);
    const t3 = setTimeout(() => onDone(), 2200);
    return () => [t1,t2,t3].forEach(clearTimeout);
  }, []);

  return (
    <motion.div animate={{ opacity: phase===2 ? 0 : 1 }} transition={{ duration:0.45 }}
      style={{ position:'fixed', inset:0, zIndex:100, background:'#09090b', display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center' }}>
      <div style={{ position:'absolute', inset:0, overflow:'hidden', opacity:0.035 }}>
        {Array.from({length:12}).map((_,i) => <div key={'h'+i} style={{ position:'absolute', left:0, right:0, top:`${(i+1)*8}%`, height:1, background:'#60a5fa' }} />)}
        {Array.from({length:16}).map((_,i) => <div key={'v'+i} style={{ position:'absolute', top:0, bottom:0, left:`${(i+1)*6.25}%`, width:1, background:'#60a5fa' }} />)}
      </div>
      <motion.div initial={{ scale:0.5, opacity:0 }} animate={{ scale:1, opacity:0.15 }} transition={{ duration:0.8 }}
        style={{ position:'absolute', width:320, height:320, borderRadius:'50%', background:'radial-gradient(circle, #60a5fa 0%, transparent 70%)' }} />
      <motion.div initial={{ opacity:0, y:16 }} animate={{ opacity:1, y:0 }} transition={{ duration:0.6 }}
        style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:14, position:'relative' }}>
        <div style={{ display:'flex', alignItems:'center', gap:14 }}>
          <motion.div animate={{ rotate:360 }} transition={{ duration:8, repeat:Infinity, ease:'linear' }}
            style={{ width:40, height:40, borderRadius:'50%', border:'1.5px solid rgba(96,165,250,0.35)', display:'flex', alignItems:'center', justifyContent:'center', position:'relative' }}>
            <div style={{ position:'absolute', width:7, height:7, borderRadius:'50%', background:'#60a5fa', top:4, left:'50%', transform:'translateX(-50%)' }} />
            <div style={{ width:16, height:16, borderRadius:'50%', background:'rgba(96,165,250,0.12)', border:'1px solid rgba(96,165,250,0.35)' }} />
          </motion.div>
          <span style={{ fontSize:30, fontWeight:800, color:'#f4f4f5', letterSpacing:'-0.03em' }}>TERMINUS</span>
        </div>
        <motion.div initial={{ opacity:0 }} animate={{ opacity: phase>=1 ? 1 : 0 }} transition={{ duration:0.4 }}
          style={{ fontSize:11, color:'#3f3f46', letterSpacing:'0.22em', textTransform:'uppercase' }}>
          Multi-Exchange Analytics
        </motion.div>
        <motion.div initial={{ opacity:0 }} animate={{ opacity: phase>=1 ? 1 : 0 }} transition={{ duration:0.3 }}
          style={{ width:180, height:1, background:'#1d1d20', borderRadius:1, overflow:'hidden', marginTop:4 }}>
          <motion.div initial={{ width:0 }} animate={{ width: phase>=1 ? '100%' : 0 }} transition={{ duration:0.9, ease:'easeInOut' }}
            style={{ height:'100%', background:'linear-gradient(90deg,#60a5fa,#a78bfa)' }} />
        </motion.div>
      </motion.div>
    </motion.div>
  );
};

// ─── Settings Drawer (выезжает сверху) ───────────────────────────────────────
const SettingsDrawer = ({ open, onClose, accentColor, title, icon, children }) => (
  <AnimatePresence>
    {open && (
      <>
        {/* backdrop — закрывает клик мимо */}
        <motion.div
          initial={{ opacity:0 }} animate={{ opacity:1 }} exit={{ opacity:0 }}
          transition={{ duration:0.18 }}
          onClick={onClose}
          style={{ position:'absolute', inset:0, zIndex:10, background:'rgba(0,0,0,0.5)', backdropFilter:'blur(2px)' }}
        />
        {/* drawer */}
        <motion.div
          initial={{ y:'-100%', opacity:0 }}
          animate={{ y:0, opacity:1 }}
          exit={{ y:'-100%', opacity:0 }}
          transition={{ type:'spring', stiffness:380, damping:34 }}
          style={{
            position:'absolute', top:0, left:0, right:0, zIndex:11,
            background:'#0e0e10', borderBottom:`1px solid ${accentColor}33`,
            boxShadow:`0 8px 32px rgba(0,0,0,0.6), 0 0 0 1px ${accentColor}1a`,
            padding:'14px 14px 16px',
          }}
        >
          <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', marginBottom:14 }}>
            <div style={{ display:'flex', alignItems:'center', gap:8 }}>
              <Ico d={icon} size={13} color={accentColor} />
              <span style={{ fontSize:11, fontWeight:700, color:accentColor, textTransform:'uppercase', letterSpacing:'0.1em' }}>{title}</span>
            </div>
            <motion.button onClick={onClose} whileTap={{ scale:0.9 }}
              style={{ display:'flex', alignItems:'center', justifyContent:'center', width:28, height:28, borderRadius:6, background:'#1a1a1d', border:'1px solid #27272a', cursor:'pointer', color:'#52525b' }}>
              <Ico d={I.close} size={13} color="#52525b" />
            </motion.button>
          </div>
          {children}
        </motion.div>
      </>
    )}
  </AnimatePresence>
);

// ─── Splash Card ──────────────────────────────────────────────────────────────
const SplashCard = ({ s }) => {
  const dc    = s.direction==='UP' ? C.up : C.down;
  const ec    = exClr(s.exchange);
  const ended = s.status==='RETURNED' || s.status==='TIMEOUT';
  const url   = getUrl(s.exchange||'MEXC', s.symbol);

  return (
    <motion.div layout
      initial={{ opacity:0, y:-10, scale:0.98 }} animate={{ opacity:ended?0.45:1, y:0, scale:1 }}
      exit={{ opacity:0, scale:0.96, transition:{ duration:0.15 } }}
      transition={{ type:'spring', stiffness:400, damping:35 }}
      style={{ background:s.isPinned&&!ended?'rgba(22,163,74,0.04)':'#111113', border:`1px solid ${s.isPinned&&!ended?'rgba(22,163,74,0.2)':'#1d1d20'}`, borderRadius:8, padding:'12px 14px' }}
    >
      <div style={{ display:'flex', alignItems:'flex-start', justifyContent:'space-between', marginBottom:10 }}>
        <div style={{ display:'flex', flexWrap:'wrap', alignItems:'center', gap:6 }}>
          <span style={{ fontSize:14, fontWeight:700, color:'#f4f4f5', letterSpacing:'-0.02em' }}>{s.symbol}</span>
          <Badge label={s.exchange||'MEXC'} c={ec} />
          <Badge label={`${s.direction} ${s.level}%`} c={dc} />
          {s.isProgression&&!ended && <Badge label="escalation" c={C.blue} />}
          {ended && <Badge label={s.status==='RETURNED'?`returned ${s.returnTime||''}s`:'timeout'} c={s.status==='RETURNED'?C.up:{t:'#f87171',bg:'rgba(239,68,68,0.08)',b:'rgba(239,68,68,0.2)'}} />}
        </div>
        <div style={{ display:'flex', alignItems:'center', gap:10, flexShrink:0, marginLeft:8 }}>
          <div style={{ textAlign:'right' }}>
            <div style={{ fontSize:9, color:'#3f3f46', marginBottom:2 }}>win prob</div>
            <div style={{ fontSize:18, fontWeight:700, color:probClr(s.prob), fontFamily:'monospace', lineHeight:1 }}>{s.prob>0?`${s.prob}%`:'—'}</div>
          </div>
          <motion.a href={url} onClick={e=>{ e.preventDefault(); BrowserOpenURL(url); }}
            whileHover={{ scale:1.08 }} whileTap={{ scale:0.93 }}
            style={{ display:'flex', alignItems:'center', justifyContent:'center', width:32, height:32, borderRadius:6, background:'#1c1c1f', border:'1px solid #27272a', color:'#52525b', textDecoration:'none', flexShrink:0 }}>
            <Ico d={I.link} size={13} color="#71717a" />
          </motion.a>
        </div>
      </div>

      <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap:6, marginBottom:10 }}>
        {[['Reference',s.refLast,s.refFair,'#3f3f46'],['Current',s.lastPrice,s.fairPrice,'#a1a1aa']].map(([lbl,last,fair,col])=>(
          <div key={lbl} style={{ background:'#0a0a0b', borderRadius:6, padding:'7px 9px', border:'1px solid #1a1a1d' }}>
            <div style={{ fontSize:9, color:'#2d2d30', marginBottom:4, textTransform:'uppercase', letterSpacing:'0.07em', fontWeight:600 }}>{lbl}</div>
            <div style={{ fontSize:11, color:col, fontFamily:'monospace', lineHeight:1.7 }}>Last: {last}</div>
            <div style={{ fontSize:11, color:col, fontFamily:'monospace', lineHeight:1.7 }}>Fair: {fair}</div>
          </div>
        ))}
      </div>

      <div style={{ display:'flex', gap:14, flexWrap:'wrap', alignItems:'center' }}>
        {s.gap>0 && <span style={{ fontSize:10, color:'#52525b' }}>gap <span style={{ color:'#a78bfa', fontWeight:600 }}>{s.gap}%</span></span>}
        <span style={{ fontSize:10, color:'#52525b' }}>speed <span style={{ color:'#facc15', fontWeight:600 }}>{s.speed}s</span></span>
        <span style={{ fontSize:10, color:'#52525b' }}>vol <span style={{ color:'#71717a', fontWeight:600 }}>{s.volume?(s.volume/1e6).toFixed(1)+'M':'—'}</span></span>
        <span style={{ fontSize:10, color:'#52525b' }}>window <span style={{ color:'#71717a', fontWeight:600 }}>{s.activeWindow}m</span></span>
        <span style={{ fontSize:10, color:'#2a2a2d', marginLeft:'auto' }}>{s.timestamp}</span>
      </div>
    </motion.div>
  );
};

// ─── Spread Card ──────────────────────────────────────────────────────────────
const SpreadCard = ({ sp }) => {
  const sc     = sprdClr(sp.spreadPct);
  const srcClr = { CEX:C.blue, DEX:C.purple, 'CEX-DEX':C.orange }[sp.source] || C.blue;

  return (
    <motion.div layout
      initial={{ opacity:0, y:-10, scale:0.98 }} animate={{ opacity:1, y:0, scale:1 }}
      exit={{ opacity:0, scale:0.96, transition:{ duration:0.15 } }}
      transition={{ type:'spring', stiffness:400, damping:35 }}
      style={{ background:sp.isAlert?'rgba(251,146,60,0.03)':'#111113', border:`1px solid ${sp.isAlert?'rgba(251,146,60,0.22)':'#1d1d20'}`, borderRadius:8, padding:'12px 14px' }}
    >
      <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', marginBottom:10 }}>
        <div style={{ display:'flex', alignItems:'center', gap:6 }}>
          <span style={{ fontSize:14, fontWeight:700, color:'#f4f4f5', letterSpacing:'-0.02em' }}>{sp.symbol}</span>
          <Badge label={sp.source} c={srcClr} />
          {sp.isAlert && <Badge label="alert" c={C.orange} />}
        </div>
        <div style={{ fontSize:22, fontWeight:700, color:sc, fontFamily:'monospace', lineHeight:1 }}>{sp.spreadPct.toFixed(2)}%</div>
      </div>

      <div style={{ display:'grid', gridTemplateColumns:'1fr 28px 1fr', gap:8, alignItems:'center', marginBottom:10 }}>
        {[
          { side:'Long', label:'Buy cheaper',  ex:sp.buyExchange,  price:sp.buyPrice,  url:getUrl(sp.buyExchange,  sp.symbol), c:C.up   },
          { side:'Short', label:'Sell higher', ex:sp.sellExchange, price:sp.sellPrice, url:getUrl(sp.sellExchange, sp.symbol), c:C.down },
        ].map((s,i)=>(
          <React.Fragment key={i}>
            {i===1 && <div style={{ display:'flex', alignItems:'center', justifyContent:'center' }}><Ico d={I.arrows} size={14} color="#2d2d30" /></div>}
            <div style={{ background:s.c.bg, border:`1px solid ${s.c.b}`, borderRadius:7, padding:'9px 11px' }}>
              <div style={{ fontSize:9, color:s.c.t, marginBottom:5, textTransform:'uppercase', letterSpacing:'0.07em', fontWeight:700 }}>{s.side} / {s.label}</div>
              <div style={{ fontSize:12, fontWeight:700, color:'#e4e4e7', marginBottom:2 }}>{s.ex}</div>
              <div style={{ fontSize:11, color:'#71717a', fontFamily:'monospace', marginBottom:8 }}>{s.price?.toPrecision(6)}</div>
              <motion.a href={s.url} onClick={e=>{ e.preventDefault(); BrowserOpenURL(s.url); }}
                whileHover={{ scale:1.02 }} whileTap={{ scale:0.95 }}
                style={{ display:'inline-flex', alignItems:'center', gap:5, fontSize:11, fontWeight:600, color:s.c.t, textDecoration:'none', border:`1px solid ${s.c.b}`, borderRadius:5, padding:'5px 10px', background:s.c.bg }}>
                Open <Ico d={I.link} size={11} color={s.c.t} />
              </motion.a>
            </div>
          </React.Fragment>
        ))}
      </div>

      <div style={{ display:'flex', gap:14 }}>
        <span style={{ fontSize:10, color:'#52525b' }}>vol <span style={{ color:'#71717a', fontWeight:600 }}>{sp.volume24h?(sp.volume24h/1e6).toFixed(1)+'M':'—'}</span></span>
        <span style={{ fontSize:10, color:'#2a2a2d', marginLeft:'auto' }}>{sp.timestamp}</span>
      </div>
    </motion.div>
  );
};

// ─── Splash Panel ─────────────────────────────────────────────────────────────
const SplashPanel = () => {
  const [signals, setSignals]          = useState([]);
  const [settingsOpen, setSettingsOpen]= useState(false);
  const [tiers, setTiers]              = useState([{ level:3, window:10, isForcedPin:false },{ level:5, window:15, isForcedPin:false }]);
  const [filterExchanges, setFilterEx] = useState([]);
  const [filterMinLevel, setFilterLvl] = useState(0);
  const [filterMinProb, setFilterProb] = useState(0);
  const tiersRef = useRef(tiers);
  useEffect(() => { tiersRef.current = tiers; }, [tiers]);

  const getTier = useCallback((lvl) => {
    const sorted = [...tiersRef.current].sort((a,b)=>a.level-b.level);
    return sorted.filter(c=>parseFloat(lvl)>=c.level).pop()||null;
  }, []);

  useEffect(() => {
    const t = setInterval(() => {
      const now = Date.now();
      setSignals(prev => {
        let ch=false;
        const next = prev.map(s => {
          if (s.isPinned&&s.unpinAt&&now>s.unpinAt) { ch=true; return {...s,isPinned:false,unpinAt:null}; }
          if (s.status==='ACTIVE') {
            const tier=getTier(s.level);
            if (now-s.createdAt>(tier?.window||s.activeWindow||5)*60000) { ch=true; return {...s,status:'TIMEOUT',deleteAt:now+8000,isPinned:false}; }
          }
          return s;
        }).filter(s => { if(s.deleteAt&&now>s.deleteAt){ch=true;return false;} return true; });
        return ch ? next : prev;
      });
    }, 1000);
    return () => clearInterval(t);
  }, [getTier]);

  useEffect(() => {
    const unsub = EventsOn('splash:new', (data) => {
      setSignals(prev => {
        const now=Date.now();
        const key=`${data.symbol}:${data.exchange||'MEXC'}`;
        const idx=prev.findIndex(s=>`${s.symbol}:${s.exchange||'MEXC'}`===key);
        if (data.status==='RETURNED'||data.status==='TIMEOUT') {
          if(idx===-1) return prev;
          const u=[...prev]; u[idx]={...u[idx],...data,isPinned:false,deleteAt:now+8000}; return u;
        }
        const tier=getTier(data.level);
        if(!tier) return prev;
        const pin=data.prob>60||tier.isForcedPin;
        if(idx!==-1){
          const ex=prev[idx];
          if(ex.status!=='ACTIVE') return prev;
          const exTier=getTier(ex.level);
          if(exTier&&tier.level>exTier.level){
            const u={...ex,...data,isProgression:true,activeWindow:tier.window,createdAt:now,isPinned:pin,unpinAt:pin?now+10000:null};
            return [u,...prev.filter((_,i)=>i!==idx)].slice(0,100);
          }
          return prev;
        }
        return [{...data,id:`${key}-${now}`,isPinned:pin,unpinAt:pin?now+10000:null,createdAt:now,activeWindow:tier.window,status:'ACTIVE'},...prev].slice(0,100);
      });
    });
    UpdateConfig({ tiers });
    return () => unsub();
  }, []);

  const toggleEx=(ex)=>setFilterEx(p=>p.includes(ex)?p.filter(e=>e!==ex):[...p,ex]);

  const displayed=[...signals]
    .filter(s=>filterExchanges.length===0||filterExchanges.includes(s.exchange||'MEXC'))
    .filter(s=>s.level>=filterMinLevel)
    .filter(s=>filterMinProb===0||s.prob>=filterMinProb)
    .sort((a,b)=>{ const ae=a.status!=='ACTIVE',be=b.status!=='ACTIVE'; if(ae!==be)return ae?1:-1; if(a.isPinned!==b.isPinned)return a.isPinned?-1:1; return b.level-a.level; });

  const activeFilters = filterExchanges.length + (filterMinLevel>0?1:0) + (filterMinProb>0?1:0);

  return (
    <div style={{ display:'flex', flexDirection:'column', height:'100%', overflow:'hidden', position:'relative' }}>

      {/* Compact header bar */}
      <div style={{ display:'flex', alignItems:'center', gap:8, padding:'0 12px', height:44, borderBottom:'1px solid #1a1a1d', background:'#0c0c0e', flexShrink:0 }}>
        <Ico d={I.zap} size={13} color="#60a5fa" />
        <span style={{ fontSize:11, fontWeight:700, color:'#60a5fa', textTransform:'uppercase', letterSpacing:'0.1em' }}>Splash</span>
        <span style={{ fontSize:10, color:'#27272a', marginLeft:4 }}>{displayed.length} signals</span>
        <div style={{ flex:1 }} />

        {/* Filter chips — compact row */}
        <div style={{ display:'flex', gap:3, alignItems:'center' }}>
          {[[0,'all'],...LEVELS.map(l=>[l,l+'%'])].map(([v,l]) => (
            <motion.button key={v} onClick={()=>setFilterLvl(v)} whileTap={{scale:0.92}}
              style={{ padding:'3px 7px', fontSize:10, fontWeight:600, borderRadius:4, cursor:'pointer', border:'1px solid', background:filterMinLevel===v?'rgba(96,165,250,0.12)':'transparent', borderColor:filterMinLevel===v?'rgba(96,165,250,0.4)':'#1d1d20', color:filterMinLevel===v?'#60a5fa':'#3f3f46', transition:'all 0.1s' }}>
              {l}
            </motion.button>
          ))}
        </div>

        <motion.button onClick={()=>setSettingsOpen(true)} whileTap={{scale:0.93}}
          style={{ display:'flex', alignItems:'center', gap:6, padding:'5px 10px', borderRadius:6, background: activeFilters>0?'rgba(96,165,250,0.1)':'#1a1a1d', border:`1px solid ${activeFilters>0?'rgba(96,165,250,0.35)':'#27272a'}`, color:activeFilters>0?'#60a5fa':'#52525b', cursor:'pointer', fontSize:11, fontWeight:600 }}>
          <Ico d={I.settings} size={13} color={activeFilters>0?'#60a5fa':'#52525b'} />
          {activeFilters>0 && <span style={{ fontSize:10, background:'#2563eb', color:'#fff', borderRadius:'50%', width:16, height:16, display:'flex', alignItems:'center', justifyContent:'center', fontWeight:700 }}>{activeFilters}</span>}
        </motion.button>
      </div>

      {/* Settings drawer */}
      <SettingsDrawer open={settingsOpen} onClose={()=>setSettingsOpen(false)} accentColor="#60a5fa" title="Splash Settings" icon={I.zap}>
        {/* Tiers */}
        <div style={{ marginBottom:14 }}>
          <div style={{ fontSize:10, color:'#3f3f46', textTransform:'uppercase', letterSpacing:'0.08em', fontWeight:600, marginBottom:8 }}>Tiers</div>
          <div style={{ display:'flex', flexDirection:'column', gap:6 }}>
            {tiers.map((t,i)=>(
              <div key={i} style={{ display:'grid', gridTemplateColumns:'1fr 1fr auto auto', gap:6, alignItems:'end' }}>
                <NumInput label="Level %" value={t.level} onChange={v=>{const n=[...tiers];n[i]={...n[i],level:v};setTiers(n);}} />
                <NumInput label="Window m" value={t.window} onChange={v=>{const n=[...tiers];n[i]={...n[i],window:v};setTiers(n);}} />
                <div style={{ display:'flex', flexDirection:'column', gap:4 }}>
                  <label style={{ fontSize:10, color:'transparent', userSelect:'none' }}>x</label>
                  <motion.button onClick={()=>{const n=[...tiers];n[i]={...n[i],isForcedPin:!n[i].isForcedPin};setTiers(n);}} whileTap={{scale:0.92}}
                    style={{ height:34, width:36, borderRadius:6, background:t.isForcedPin?'rgba(59,130,246,0.15)':'#1a1a1d', border:`1px solid ${t.isForcedPin?'#3b82f6':'#27272a'}`, cursor:'pointer', display:'flex', alignItems:'center', justifyContent:'center' }}>
                    <Ico d={t.isForcedPin?I.pin:I.pinOff} size={13} color={t.isForcedPin?'#60a5fa':'#3f3f46'} />
                  </motion.button>
                </div>
                <div style={{ display:'flex', flexDirection:'column', gap:4 }}>
                  <label style={{ fontSize:10, color:'transparent', userSelect:'none' }}>x</label>
                  <motion.button onClick={()=>setTiers(tiers.filter((_,j)=>j!==i))} whileTap={{scale:0.92}}
                    style={{ height:34, width:36, borderRadius:6, background:'#1a1a1d', border:'1px solid #27272a', cursor:'pointer', display:'flex', alignItems:'center', justifyContent:'center' }}>
                    <Ico d={I.trash} size={13} color="#52525b" />
                  </motion.button>
                </div>
              </div>
            ))}
          </div>
          <div style={{ display:'flex', gap:6, marginTop:8 }}>
            <motion.button onClick={()=>setTiers([...tiers,{level:5,window:5,isForcedPin:false}])} whileTap={{scale:0.95}}
              style={{ flex:1, padding:'7px 0', fontSize:11, background:'transparent', border:'1px dashed #27272a', borderRadius:6, color:'#3f3f46', cursor:'pointer', display:'flex', alignItems:'center', justifyContent:'center', gap:6 }}>
              <Ico d={I.plus} size={12} color="#3f3f46" /> Add tier
            </motion.button>
            <motion.button onClick={()=>{UpdateConfig({tiers});setSettingsOpen(false);}} whileTap={{scale:0.95}}
              style={{ flex:1, padding:'7px 0', fontSize:11, fontWeight:600, background:'rgba(59,130,246,0.15)', border:'1px solid rgba(59,130,246,0.4)', borderRadius:6, color:'#60a5fa', cursor:'pointer' }}>
              Apply
            </motion.button>
          </div>
        </div>

        {/* Filters */}
        <div style={{ borderTop:'1px solid #1a1a1d', paddingTop:12 }}>
          <div style={{ fontSize:10, color:'#3f3f46', textTransform:'uppercase', letterSpacing:'0.08em', fontWeight:600, marginBottom:8 }}>Filters</div>
          <div style={{ display:'flex', flexWrap:'wrap', gap:4, marginBottom:8 }}>
            {EXCHANGES.map(ex=><Chip key={ex} label={ex} active={filterExchanges.includes(ex)} onClick={()=>toggleEx(ex)} color={exClr(ex)} />)}
          </div>
          <div style={{ display:'flex', flexWrap:'wrap', gap:4, alignItems:'center', marginBottom:8 }}>
            <span style={{ fontSize:10, color:'#3f3f46', marginRight:2 }}>prob ≥</span>
            {[[0,'all'],[30,'30%'],[50,'50%'],[60,'60%'],[75,'75%']].map(([v,l])=><Chip key={v} label={l} active={filterMinProb===v} onClick={()=>setFilterProb(v)} />)}
          </div>
          {(filterExchanges.length>0||filterMinProb>0) && (
            <motion.button initial={{opacity:0}} animate={{opacity:1}} onClick={()=>{setFilterEx([]);setFilterProb(0);}} whileTap={{scale:0.95}}
              style={{ fontSize:11, color:'#f87171', background:'transparent', border:'1px solid rgba(248,113,113,0.2)', borderRadius:5, padding:'4px 10px', cursor:'pointer' }}>
              Clear filters
            </motion.button>
          )}
        </div>
      </SettingsDrawer>

      {/* Feed */}
      <div style={{ flex:1, overflowY:'auto', padding:'8px 10px', display:'flex', flexDirection:'column', gap:6 }}>
        <AnimatePresence mode="popLayout">
          {displayed.map(s=><SplashCard key={s.id} s={s} />)}
        </AnimatePresence>
        {displayed.length===0 && (
          <motion.div initial={{opacity:0}} animate={{opacity:1}} transition={{delay:0.3}}
            style={{ display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', height:'100%', gap:12, color:'#27272a' }}>
            <Ico d={I.zap} size={32} color="#27272a" sw={1} />
            <span style={{ fontSize:11, textTransform:'uppercase', letterSpacing:'0.1em' }}>Waiting for splashes</span>
          </motion.div>
        )}
      </div>
    </div>
  );
};

// ─── Spread Panel ─────────────────────────────────────────────────────────────
const SpreadPanel = () => {
  const [spreads, setSpreads]          = useState([]);
  const [settingsOpen, setSettingsOpen]= useState(false);
  const [filter, setFilter]            = useState('ALL');
  const [cfg, setCfg]                  = useState({ alertThresholdPct:1.0, minVolume24h:500000, enableCex:true, enableDex:true, pollingIntervalMs:2000 });

  useEffect(() => {
    const unsub = EventsOn('spread:new', (data) => {
      setSpreads(prev => {
        const key=`${data.symbol}:${data.buyExchange}:${data.sellExchange}`;
        return [data,...prev.filter(s=>`${s.symbol}:${s.buyExchange}:${s.sellExchange}`!==key)].slice(0,200);
      });
    });
    GetLatestSpreads().then(d=>{if(d?.length)setSpreads(d);}).catch(()=>{});
    return () => unsub();
  }, []);

  const alertCount = spreads.filter(s=>s.isAlert).length;
  const displayed  = spreads.filter(s=>filter==='ALL'||s.source===filter).sort((a,b)=>b.spreadPct-a.spreadPct);

  return (
    <div style={{ display:'flex', flexDirection:'column', height:'100%', overflow:'hidden', position:'relative' }}>

      {/* Compact header bar */}
      <div style={{ display:'flex', alignItems:'center', gap:8, padding:'0 12px', height:44, borderBottom:'1px solid #1a1a1d', background:'#0c0c0e', flexShrink:0 }}>
        <Ico d={I.arrows} size={13} color="#fb923c" />
        <span style={{ fontSize:11, fontWeight:700, color:'#fb923c', textTransform:'uppercase', letterSpacing:'0.1em' }}>Spread</span>
        <span style={{ fontSize:10, color:'#27272a', marginLeft:4 }}>{displayed.length} pairs</span>
        {alertCount>0 && <span style={{ fontSize:10, color:'#fb923c' }}>· {alertCount} alerts</span>}
        <div style={{ flex:1 }} />

        {/* Source filter chips */}
        <div style={{ display:'flex', gap:3 }}>
          {['ALL','CEX','DEX','CEX-DEX'].map(f=>(
            <motion.button key={f} onClick={()=>setFilter(f)} whileTap={{scale:0.92}}
              style={{ padding:'3px 8px', fontSize:10, fontWeight:600, borderRadius:4, cursor:'pointer', border:'1px solid', background:filter===f?({CEX:'rgba(96,165,250,0.12)',DEX:'rgba(167,139,250,0.12)','CEX-DEX':'rgba(251,146,60,0.12)',ALL:'rgba(255,255,255,0.06)'}[f]):'transparent', borderColor:filter===f?({CEX:'rgba(96,165,250,0.4)',DEX:'rgba(167,139,250,0.4)','CEX-DEX':'rgba(251,146,60,0.4)',ALL:'rgba(255,255,255,0.15)'}[f]):'#1d1d20', color:filter===f?({CEX:'#60a5fa',DEX:'#a78bfa','CEX-DEX':'#fb923c',ALL:'#a1a1aa'}[f]):'#3f3f46', transition:'all 0.1s' }}>
              {f}
            </motion.button>
          ))}
        </div>

        <motion.button onClick={()=>setSettingsOpen(true)} whileTap={{scale:0.93}}
          style={{ display:'flex', alignItems:'center', gap:6, padding:'5px 10px', borderRadius:6, background:'#1a1a1d', border:'1px solid #27272a', color:'#52525b', cursor:'pointer', fontSize:11, fontWeight:600 }}>
          <Ico d={I.settings} size={13} color="#52525b" />
        </motion.button>
      </div>

      {/* Settings drawer */}
      <SettingsDrawer open={settingsOpen} onClose={()=>setSettingsOpen(false)} accentColor="#fb923c" title="Spread Settings" icon={I.arrows}>
        <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap:8, marginBottom:10 }}>
          <NumInput label="Alert %" value={cfg.alertThresholdPct} onChange={v=>setCfg(c=>({...c,alertThresholdPct:v}))} suffix="%" />
          <NumInput label="Min volume $" value={cfg.minVolume24h} onChange={v=>setCfg(c=>({...c,minVolume24h:v}))} />
          <NumInput label="Interval ms" value={cfg.pollingIntervalMs} onChange={v=>setCfg(c=>({...c,pollingIntervalMs:v}))} suffix="ms" />
        </div>
        <div style={{ display:'flex', flexDirection:'column', gap:8, marginBottom:12 }}>
          <Toggle label="CEX — MEXC · OKX · Gate.io · BingX" value={cfg.enableCex} onChange={v=>setCfg(c=>({...c,enableCex:v}))} />
          <Toggle label="DEX — Hyperliquid" value={cfg.enableDex} onChange={v=>setCfg(c=>({...c,enableDex:v}))} />
        </div>
        <motion.button onClick={()=>{UpdateSpreadConfig(cfg);setSettingsOpen(false);}} whileTap={{scale:0.97}}
          style={{ width:'100%', padding:'9px 0', fontSize:12, fontWeight:600, background:'rgba(251,146,60,0.12)', border:'1px solid rgba(251,146,60,0.35)', borderRadius:6, color:'#fb923c', cursor:'pointer' }}>
          Apply
        </motion.button>
      </SettingsDrawer>

      {/* Feed */}
      <div style={{ flex:1, overflowY:'auto', padding:'8px 10px', display:'flex', flexDirection:'column', gap:6 }}>
        <AnimatePresence mode="popLayout">
          {displayed.map(sp=><SpreadCard key={`${sp.symbol}:${sp.buyExchange}:${sp.sellExchange}`} sp={sp} />)}
        </AnimatePresence>
        {displayed.length===0 && (
          <motion.div initial={{opacity:0}} animate={{opacity:1}} transition={{delay:0.3}}
            style={{ display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', height:'100%', gap:12, color:'#27272a' }}>
            <Ico d={I.arrows} size={32} color="#27272a" sw={1} />
            <span style={{ fontSize:11, textTransform:'uppercase', letterSpacing:'0.1em' }}>Scanning spreads</span>
          </motion.div>
        )}
      </div>
    </div>
  );
};

// ─── Root ─────────────────────────────────────────────────────────────────────
export default function App() {
  const [ready, setReady] = useState(false);
  return (
    <>
      <AnimatePresence>{!ready && <SplashScreen onDone={()=>setReady(true)} />}</AnimatePresence>
      <motion.div initial={{opacity:0}} animate={{opacity:ready?1:0}} transition={{duration:0.35}}
        style={{ height:'100vh', width:'100vw', background:'#09090b', color:'#e4e4e7', fontFamily:'-apple-system,"Inter",sans-serif', display:'flex', flexDirection:'column', overflow:'hidden' }}>
        <div style={{ height:38, borderBottom:'1px solid #1a1a1d', background:'#0c0c0e', display:'flex', alignItems:'center', padding:'0 16px', gap:14, flexShrink:0 }}>
          <div style={{ display:'flex', alignItems:'center', gap:9 }}>
            <motion.div animate={{rotate:360}} transition={{duration:12,repeat:Infinity,ease:'linear'}}
              style={{ width:14, height:14, borderRadius:'50%', border:'1.5px solid rgba(96,165,250,0.4)', position:'relative', display:'flex', alignItems:'center', justifyContent:'center' }}>
              <div style={{ width:4, height:4, borderRadius:'50%', background:'#60a5fa' }} />
            </motion.div>
            <span style={{ fontSize:12, fontWeight:700, color:'#e4e4e7', letterSpacing:'0.06em' }}>TERMINUS</span>
          </div>
          <span style={{ fontSize:10, color:'#27272a', textTransform:'uppercase', letterSpacing:'0.06em' }}>Multi-Exchange Analytics</span>
          <div style={{ marginLeft:'auto', display:'flex', alignItems:'center', gap:6 }}>
            <motion.div animate={{opacity:[1,0.3,1]}} transition={{duration:2,repeat:Infinity}}
              style={{ width:6, height:6, borderRadius:'50%', background:'#22c55e' }} />
            <span style={{ fontSize:10, color:'#3f3f46' }}>live</span>
          </div>
        </div>
        <div style={{ flex:1, display:'grid', gridTemplateColumns:'1fr 1px 1fr', overflow:'hidden' }}>
          <SplashPanel />
          <div style={{ background:'#1a1a1d' }} />
          <SpreadPanel />
        </div>
      </motion.div>
    </>
  );
}
