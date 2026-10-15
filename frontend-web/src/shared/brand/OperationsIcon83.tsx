const paths:Record<string,string>={
 closing:'M4 9h16v12H4z M7 9V5h10v4 M8 14h8 M9 18h6',
 journal:'M5 3h14v18H5z M9 7h6 M9 11h6 M9 15h6 M9 19h4',
 returns:'M8 7H3V2 M3 7a9 9 0 1 1-1 8 M8 12h9 M13 8l4 4-4 4',
 inventory:'M3 7l9-4 9 4-9 5z M3 7v12l9 4 9-4V7 M12 12v11 M7 9l9-4',
 quality:'M8 3h8 M10 3v6L4 19q-1 2 2 2h12q3 0 2-2L14 9V3 M7 15h10 M10 18h1',
 supply:'M2 5h12v13H2z M14 10h4l4 5v3h-8 M5 18a2 2 0 1 0 4 0 M16 18a2 2 0 1 0 4 0',
 opportunities:'M12 3a7 7 0 0 0-4 13v4h8v-4a7 7 0 0 0-4-13 M9 23h6 M12 7v6 M9 10h6',
 performance:'M3 3v18h18 M7 17v-5 M12 17V7 M17 17V4',
 devices:'M2 3h14v11H2z M6 18h7 M9 14v4 M18 9h4v12h-4z',
 operations83:'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M17 14v7 M14 17h7'
};
export default function OperationsIcon83({area='operations83'}:{area?:string}){return <svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d={paths[area]||paths.operations83}/></svg>;}
