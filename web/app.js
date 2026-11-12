"use strict";

// ---------------------------------------------------------------------------
// Paramètres
// ---------------------------------------------------------------------------
const ENDPOINT = window.GOOGLE_SHEETS_ENDPOINT;
const toneLevel = 0.08, tone1Dur = 600, tone2Dur = 600, gapDur = 500;
const FEEDBACK_MS = 450;          // durée d'affichage du « ✓ Réponse enregistrée »
const N_EXP1 = 30;                // essais expérience 1 (notes identiques / différentes)
const N_EXP2 = 30;                // essais expérience 2 (sons proches)
const N_IDENT = 30;               // essais d'identification des notes
const noteFreqs = [262.29, 294.63, 330.87, 350.54, 392.98, 441.51, 495.55, 524.42];
const noteNames = ["C4","D4","E4","F4","G4","A4","B4","C5"];
const noteSemis = [0,2,4,5,7,9,11,12];

const screenEl = document.querySelector("#screen");
const isTouch = window.matchMedia("(hover: none)").matches;
const sessionId = crypto.randomUUID();
let participantId = "";
let rows = [], queue = [], stats = {}, continuum, lowLabel, highLabel, sameKey, diffKey, lowKey, highKey;

function shuffle(a) { return a.sort(() => Math.random() - .5); }
const sleep = ms => new Promise(r => setTimeout(r, ms));

// ---------------------------------------------------------------------------
// Audio
// ---------------------------------------------------------------------------
let audio;
function ensureAudio() {
  if (!audio) audio = new (window.AudioContext || window.webkitAudioContext)();
  if (audio.state === "suspended") audio.resume();
}
function playTone(freq, duration = tone1Dur) {
  ensureAudio();
  const now = audio.currentTime, gain = audio.createGain(), osc = audio.createOscillator();
  osc.type = "triangle"; osc.frequency.value = freq; gain.gain.setValueAtTime(0, now);
  gain.gain.linearRampToValueAtTime(toneLevel, now + .015);
  gain.gain.exponentialRampToValueAtTime(.001, now + duration / 1000);
  osc.connect(gain).connect(audio.destination); osc.start(now); osc.stop(now + duration / 1000 + .02);
}

// ---------------------------------------------------------------------------
// Interface : chaque écran a un texte et des boutons, chaque bouton a une
// touche clavier équivalente. Les réponses ne sont acceptées que lorsque les
// boutons sont actifs ; les touches maintenues (répétition) sont ignorées.
// ---------------------------------------------------------------------------
const KEY_NAMES = { SPACE: "Espace", ENTER: "Entrée" };
let keyHandler = null;
window.addEventListener("keydown", e => {
  if (e.repeat || !keyHandler) return;
  const k = e.key === " " ? "SPACE" : e.key === "Enter" ? "ENTER" : e.key.toUpperCase();
  ensureAudio();
  if (keyHandler(k)) e.preventDefault();
});

function el(tag, cls, text) {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  if (text !== undefined) node.textContent = text;
  return node;
}

function view({ progress, title, text, options = [], layout = "" }) {
  screenEl.replaceChildren();
  if (progress) {
    const p = el("div", "progress");
    p.append(el("span", "progress-label", `${progress.label} — ${progress.i}/${progress.n}`));
    const bar = el("div", "bar"), fill = el("div", "fill");
    fill.style.width = `${Math.round(100 * progress.i / progress.n)}%`;
    bar.append(fill); p.append(bar); screenEl.append(p);
  }
  if (title) screenEl.append(el("h2", "title", title));
  const textEl = el("div", "text", text || "");
  const status = el("div", "status");
  const choices = el("div", `choices ${layout}`);
  const buttons = {};
  let onPick = null;
  for (const o of options) {
    const b = el("button", "choice"); b.type = "button"; b.disabled = true;
    b.append(el("span", "label", o.label));
    if (o.sub) b.append(el("span", "sub", o.sub));
    b.append(el("kbd", "key", KEY_NAMES[o.key] || o.key));
    b.addEventListener("click", () => { b.blur(); ensureAudio(); if (onPick) onPick(o.key); });
    buttons[o.key] = b; choices.append(b);
  }
  screenEl.append(textEl, status, choices);

  const v = {
    buttons, textEl,
    setStatus(t, cls = "") { status.textContent = t; status.className = `status ${cls}`; },
    // Attend un clic ou une touche. timeout en ms (0 = pas de limite).
    wait(timeout = 0, feedback = true) {
      return new Promise(resolve => {
        const keys = Object.keys(buttons), started = performance.now();
        let done = false, timer;
        const pick = key => {
          if (done) return false;
          done = true; clearTimeout(timer); onPick = null; keyHandler = null;
          Object.values(buttons).forEach(b => b.disabled = true);
          if (feedback) {
            if (key) { buttons[key].classList.add("selected"); v.setStatus("✓ Réponse enregistrée", "ok"); }
            else v.setStatus("Temps écoulé : pas de réponse", "late");
          }
          resolve({ key, rt: key ? performance.now() - started : -1 });
          return true;
        };
        onPick = pick;
        keyHandler = k => keys.includes(k) && pick(k);
        Object.values(buttons).forEach(b => b.disabled = false);
        if (feedback) v.setStatus("À vous !", "go");
        if (timeout) timer = setTimeout(() => pick(null), timeout);
      });
    }
  };
  return v;
}

async function instruction(title, text) {
  await view({ title, text, options: [{ key: "SPACE", label: "Continuer" }] }).wait(0, false);
}

// ---------------------------------------------------------------------------
// Enregistrement : les réponses sont envoyées après chaque phase, et aussi
// si la personne quitte ou change d'onglet en cours de route.
// ---------------------------------------------------------------------------
function logRow(row) {
  const r = { session: sessionId, participant: participantId, continuum, key_mapping: `${sameKey}=same;${diffKey}=diff`, ...row };
  rows.push(r); queue.push(r);
}
async function flush(onLeave = false) {
  if (!ENDPOINT || queue.length === 0) return true;
  const batch = queue.splice(0);
  const body = JSON.stringify({ session: sessionId, participant: participantId, rows: batch });
  if (onLeave && navigator.sendBeacon &&
      navigator.sendBeacon(ENDPOINT, new Blob([body], { type: "text/plain;charset=utf-8" }))) return true;
  try {
    await fetch(ENDPOINT, { method: "POST", mode: "no-cors", keepalive: true,
      headers: { "Content-Type": "text/plain;charset=utf-8" }, body });
    return true;
  } catch (e) {
    queue.unshift(...batch);   // réseau indisponible : on réessaiera au prochain envoi
    return false;
  }
}
window.addEventListener("pagehide", () => flush(true));
document.addEventListener("visibilitychange", () => { if (document.visibilityState === "hidden") flush(true); });

// ---------------------------------------------------------------------------
// Essais
// ---------------------------------------------------------------------------
function setup() {
  const cd = Math.random() < .5; continuum = cd ? "C4-D4" : "A4-B4";
  const freqs = cd ? [262.29,266.35,270.27,274.17,278.23,282.36,286.42,290.49,294.63] :
    [441.51,448.17,454.93,461.60,468.35,475.18,481.87,488.69,495.55];
  window.contFreqs = freqs; lowLabel = cd ? "do" : "la"; highLabel = cd ? "ré" : "si";
  const swapped = Math.random() < .5; sameKey = swapped ? "J" : "F"; diffKey = swapped ? "F" : "J";
  lowKey = sameKey; highKey = diffKey;
}
const byKey = (a, b) => a.key.localeCompare(b.key);   // F à gauche, J à droite, comme sur le clavier

async function pairTrial(f1, f2, duration, timeout, progress) {
  const v = view({ progress, text: "Les deux sons sont-ils identiques ?",
    options: [{ key: sameKey, label: "Identique" }, { key: diffKey, label: "Différente" }].sort(byKey) });
  v.setStatus("Écoutez…", "listen");
  await sleep(500); playTone(f1, duration); await sleep(duration + gapDur);
  playTone(f2, duration);
  const r = await v.wait(timeout);
  await sleep(FEEDBACK_MS);
  return r;
}
async function singleTrial(f, timeout, progress) {
  const v = view({ progress, text: "Quelle note avez-vous entendue ?",
    options: [{ key: lowKey, label: lowLabel }, { key: highKey, label: highLabel }].sort(byKey) });
  v.setStatus("Écoutez…", "listen");
  await sleep(500); playTone(f, tone2Dur);
  const r = await v.wait(timeout);
  await sleep(FEEDBACK_MS);
  return r;
}
const RATINGS = [["D","1","Très proches"],["F","2","Plutôt proches"],["G","3","Moyennement éloignées"],["H","4","Plutôt éloignées"],["J","5","Très éloignées"]];
async function distanceTrial(f1, f2, progress) {
  const v = view({ progress, text: "Quelle distance entre les deux notes ?", layout: "scale",
    options: RATINGS.map(([key, label, sub]) => ({ key, label, sub })) });
  v.setStatus("Écoutez…", "listen");
  await sleep(500); playTone(f1); await sleep(tone1Dur + gapDur); playTone(f2); await sleep(tone1Dur);
  const r = await v.wait(0);
  await sleep(FEEDBACK_MS);
  return r;
}

function score(key, same) { const answered = !!key; return { response_key: key || "none", response: key === sameKey ? "identique" : key ? "different" : "aucune_reponse", correct: answered ? (key === sameKey) === same ? 1 : 0 : -1 }; }
async function runPairPhase(name, label, trials, duration, timeout) {
  stats[name] = { total: trials.length, answered: 0, correct: 0 };
  for (let i = 0; i < trials.length; i++) {
    const t = trials[i], r = await pairTrial(t.f1, t.f2, duration, timeout, { label, i: i + 1, n: trials.length }), s = score(r.key, t.same);
    stats[name].answered += r.key ? 1 : 0; stats[name].correct += s.correct === 1 ? 1 : 0;
    logRow({ phase: name, trial: i + 1, stim1: t.l1 || t.stim1 || "", stim2: t.l2 || t.stim2 || "", f1: t.f1, f2: t.f2, semitones: t.semis ?? -1, degree: t.degree ?? -1, pair_type: t.same ? "same" : (t.pairType || "diff"), ...s, rt_ms: r.rt, rating_1to5: -1 });
  }
  flush();
}

// ---------------------------------------------------------------------------
// Déroulement
// ---------------------------------------------------------------------------
async function askParticipantId() {
  const v = view({ title: "Identifiant",
    text: "Indiquez un identifiant personnel (par exemple vos initiales suivies de deux chiffres).\nN'indiquez pas votre nom complet.",
    options: [{ key: "ENTER", label: "Continuer" }] });
  const input = el("input", "id-input");
  Object.assign(input, { type: "text", maxLength: 40, autocomplete: "off", placeholder: "ex. : JD27" });
  input.setAttribute("autocapitalize", "characters");
  v.textEl.after(input);
  input.focus();
  while (true) {
    await v.wait(0, false);
    const value = input.value.trim().slice(0, 40);
    if (value) return value;
    v.setStatus("Merci d'indiquer un identifiant.", "late");
    input.focus();
  }
}

async function main() {
  if (!ENDPOINT) throw new Error("L'URL Google Sheets n'est pas configurée.");
  setup();
  participantId = await askParticipantId();
  await instruction("Bienvenue", "Vous allez écouter des sons et répondre avec les boutons à l'écran ou avec les touches du clavier indiquées sur les boutons.\n\nDurée totale : environ 7 minutes.\nUtilisez un casque ou des écouteurs. Répondez spontanément.\n\nVos réponses sont enregistrées au fur et à mesure.");

  const profile = {};
  const questions = [["instrument","Avez-vous déjà joué d'un instrument ou pratiqué le chant ?"],["solfege","Avez-vous suivi des cours de solfège ?"],["years","Avez-vous pratiqué la musique pendant au moins trois ans ?"],["frenchNative","Le français est-il votre langue maternelle ?"],["hearingOK","Pensez-vous avoir une audition normale ?"]];
  for (let i = 0; i < questions.length; i++) {
    const [key, q] = questions[i];
    const r = await view({ progress: { label: "Questionnaire", i: i + 1, n: questions.length }, text: q,
      options: [{ key: "O", label: "Oui" }, { key: "N", label: "Non" }] }).wait();
    profile[key] = r.key === "O" ? "oui" : "non";
    await sleep(FEEDBACK_MS);
  }
  logRow({ phase: "questionnaire", trial: 0, profile, device: isTouch ? "tactile" : "ordinateur", user_agent: navigator.userAgent });
  flush();

  const vol = view({ title: "Réglage du volume",
    text: "Appuyez sur « Écouter un son » pour entendre un son de test, puis réglez le volume de votre appareil à un niveau confortable.\n\nAppuyez sur « Continuer » quand le volume vous convient." + (isTouch ? "\n\nSur iPhone, désactivez le mode silencieux." : ""),
    options: [{ key: "SPACE", label: "Écouter un son" }, { key: "ENTER", label: "Continuer" }] });
  while ((await vol.wait(0, false)).key === "SPACE") playTone(440, 800);

  const practice = shuffle([1,1,3,3,5,5].map((i, n) => ({ f1: window.contFreqs[i-1], f2: n % 2 ? window.contFreqs[i] : window.contFreqs[i-1], same: !(n % 2) })));
  await instruction("Entraînement", "Vous allez entendre deux sons.\nIndiquez s'ils sont identiques ou différents, après le deuxième son.\n\nRépondez vite : le temps de réponse est limité.");
  await runPairPhase("practice", "Entraînement", practice, tone1Dur, 1500);

  const e1 = [];
  noteFreqs.forEach((f, i) => { for (let n=0;n<7;n++) e1.push({f1:f,f2:f,l1:noteNames[i],l2:noteNames[i],same:true}); });
  for (let i=0;i<8;i++) for (let j=i+1;j<8;j++) e1.push({f1:noteFreqs[i],f2:noteFreqs[j],l1:noteNames[i],l2:noteNames[j],semis:noteSemis[j]-noteSemis[i],degree:j-i+1,same:false});
  await instruction("Expérience 1 : notes identiques ou différentes", "Les deux sons sont-ils identiques ou différents ?\n\nRépondez vite et précisément.");
  await runPairPhase("exp1_discrimination", "Expérience 1", shuffle(e1).slice(0, N_EXP1), tone1Dur, 3000);

  const e2 = []; [1,3,5,7,9].forEach(i => { for(let n=0;n<8;n++) e2.push({i1:i,i2:i,same:true}); });
  for(let i=1;i<=7;i+=2) for(let n=0;n<5;n++) { e2.push({i1:i,i2:i+2,same:false,pairType:`D${i}${i+2}`},{i1:i+2,i2:i,same:false,pairType:`D${i}${i+2}`}); }
  await instruction("Expérience 2 : sons proches", "Même consigne : les deux sons sont-ils identiques ou différents ?");
  await runPairPhase("exp2_discrimination", "Expérience 2", shuffle(e2).slice(0, N_EXP2).map(t => ({...t,f1:window.contFreqs[t.i1-1],f2:window.contFreqs[t.i2-1],stim1:t.i1,stim2:t.i2})), tone2Dur, 3000);

  const ids = shuffle(Array.from({length:9},(_,i)=>i+1).flatMap(i=>Array(10).fill(i))).slice(0, N_IDENT);
  await instruction("Identification des notes", `Vous allez entendre une seule note.\nEst-ce un « ${lowLabel} » ou un « ${highLabel} » ?\n\nRépondez avec votre première impression.`);
  for (let i=0;i<ids.length;i++) {
    const idx = ids[i], r = await singleTrial(window.contFreqs[idx-1], 5000, { label: "Identification", i: i + 1, n: ids.length });
    logRow({phase:"exp2_identification",trial:i+1,stim1:idx,f1:window.contFreqs[idx-1],response_key:r.key||"none",response:r.key===lowKey?lowLabel:r.key===highKey?highLabel:"aucune_reponse",rt_ms:r.rt,correct:-1,rating_1to5:-1});
  }
  flush();

  const e3=[]; [1,2,3,4,5,7,12].forEach(n=>{const ratio=2**(n/12); e3.push({f1:262.29,f2:262.29*ratio,semis:n,dir:"up"},{f1:330.87,f2:330.87*ratio,semis:n,dir:"up"},{f1:524.42,f2:524.42/ratio,semis:n,dir:"down"});});
  await instruction("Expérience 3 : distance entre les notes", "Vous allez entendre deux notes.\nIndiquez si elles vous semblent proches ou éloignées, de 1 (très proches) à 5 (très éloignées).");
  for (let i=0;i<e3.length;i++) {
    const r = await distanceTrial(e3[i].f1, e3[i].f2, { label: "Expérience 3", i: i + 1, n: e3.length });
    logRow({phase:"exp3_distance",trial:i+1,f1:e3[i].f1,f2:e3[i].f2,semitones:e3[i].semis,pair_type:e3[i].dir,response_key:r.key,response:r.key,rt_ms:r.rt,correct:-1,rating_1to5:RATINGS.findIndex(x => x[0] === r.key)+1});
  }
  logRow({ phase: "fin", trial: 0 });

  view({ title: "Envoi des réponses…", text: "Merci de ne pas fermer la page." });
  while (!(await flush())) {
    await view({ title: "Envoi impossible", text: "Vos réponses n'ont pas pu être envoyées. Vérifiez votre connexion Internet puis réessayez.\nNe fermez pas la page.",
      options: [{ key: "ENTER", label: "Réessayer" }] }).wait(0, false);
  }
  view({ title: "Merci pour votre participation !", text: "Vos réponses ont été enregistrées. Vous pouvez fermer cette page." });
}

view({ title: "Bienvenue", text: "Cette expérience utilise le son. Utilisez un casque ou des écouteurs.",
  options: [{ key: "ENTER", label: "Commencer" }] })
  .wait(0, false)
  .then(() => { ensureAudio(); return main(); })
  .catch(e => view({ title: "Erreur", text: e.message }));
