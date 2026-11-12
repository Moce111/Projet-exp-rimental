const screen = document.querySelector("#screen");
const toneLevel = 0.08, tone1Dur = 600, tone2Dur = 600, gapDur = 500;
const noteFreqs = [262.29, 294.63, 330.87, 350.54, 392.98, 441.51, 495.55, 524.42];
const noteNames = ["C4","D4","E4","F4","G4","A4","B4","C5"];
const noteSemis = [0,2,4,5,7,9,11,12];
const sessionId = crypto.randomUUID();
let audio, rows = [], stats = {}, continuum, lowLabel, highLabel, sameKey, diffKey, lowKey, highKey;

function shuffle(a) { return a.sort(() => Math.random() - .5); }
function show(text, buttons = []) {
  screen.innerHTML = `<div>${text}</div><div class="actions"></div>`;
  const box = screen.querySelector(".actions");
  buttons.forEach(([label, fn]) => { const b = document.createElement("button"); b.textContent = label; b.onclick = fn; box.append(b); });
}
function waitKey(keys) {
  return new Promise(resolve => {
    const handler = e => { const k = e.key.toUpperCase(); if (keys.includes(k)) { e.preventDefault(); window.removeEventListener("keydown", handler); resolve(k); } };
    window.addEventListener("keydown", handler);
  });
}
function playTone(freq, duration = tone1Dur) {
  if (!audio) audio = new (window.AudioContext || window.webkitAudioContext)();
  const now = audio.currentTime, gain = audio.createGain(), osc = audio.createOscillator();
  osc.type = "triangle"; osc.frequency.value = freq; gain.gain.setValueAtTime(0, now);
  gain.gain.linearRampToValueAtTime(toneLevel, now + .015);
  gain.gain.exponentialRampToValueAtTime(.001, now + duration / 1000);
  osc.connect(gain).connect(audio.destination); osc.start(now); osc.stop(now + duration / 1000 + .02);
}
const sleep = ms => new Promise(r => setTimeout(r, ms));
async function instruction(text) { show(text + "\n\nAppuyez sur ESPACE pour continuer."); await waitKey([" "]); }
function logRow(row) { rows.push({ session: sessionId, continuum, key_mapping: `${sameKey}=same;${diffKey}=diff`, ...row }); }
function setup() {
  const cd = Math.random() < .5; continuum = cd ? "C4-D4" : "A4-B4";
  const freqs = cd ? [262.29,266.35,270.27,274.17,278.23,282.36,286.42,290.49,294.63] :
    [441.51,448.17,454.93,461.60,468.35,475.18,481.87,488.69,495.55];
  window.contFreqs = freqs; lowLabel = cd ? "do" : "la"; highLabel = cd ? "ré" : "si";
  const swapped = Math.random() < .5; sameKey = swapped ? "J" : "F"; diffKey = swapped ? "F" : "J";
  lowKey = sameKey; highKey = diffKey;
}
async function pairTrial(f1, f2, duration, keys, timeout) {
  await sleep(500); playTone(f1, duration); await sleep(duration + gapDur);
  playTone(f2, duration); const started = performance.now();
  const key = await Promise.race([waitKey(keys), sleep(timeout).then(() => null)]);
  return { key, rt: key ? performance.now() - started : -1 };
}
async function singleTrial(f, keys, timeout) {
  await sleep(500); playTone(f, tone2Dur); const started = performance.now();
  const key = await Promise.race([waitKey(keys), sleep(timeout).then(() => null)]);
  return { key, rt: key ? performance.now() - started : -1 };
}
function score(key, same) { const answered = !!key; return { response_key: key || "none", response: key === sameKey ? "identique" : key ? "different" : "aucune_reponse", correct: answered ? (key === sameKey) === same ? 1 : 0 : -1 }; }
async function runPairPhase(name, trials, duration, timeout) {
  stats[name] = { total: trials.length, answered: 0, correct: 0 };
  for (let i = 0; i < trials.length; i++) {
    screen.querySelector(".progress").textContent = `${name} — essai ${i + 1}/${trials.length}`;
    const t = trials[i], r = await pairTrial(t.f1, t.f2, duration, [sameKey, diffKey], timeout), s = score(r.key, t.same);
    stats[name].answered += r.key ? 1 : 0; stats[name].correct += s.correct === 1 ? 1 : 0;
    logRow({ phase: name, trial: i + 1, stim1: t.l1 || "", stim2: t.l2 || "", f1: t.f1, f2: t.f2, semitones: t.semis ?? -1, degree: t.degree ?? -1, pair_type: t.same ? "same" : (t.pairType || "diff"), ...s, rt_ms: r.rt, rating_1to5: -1 });
  }
}
function progress(text) { screen.innerHTML = `<div class="progress">${text}</div>`; }
async function main() {
  setup();
  await instruction("Vous allez écouter des sons et répondre avec le clavier.\n\nDurée totale : environ 10 minutes.\nUtilisez un casque ou des écouteurs. Répondez spontanément.\n\nECHAP permet d'interrompre l'expérience.");
  const profile = {};
  await instruction("QUESTIONNAIRE PRELIMINAIRE\n\nRépondez avec O (oui) ou N (non).");
  for (const [key, q] of [["instrument","Avez-vous déjà joué d'un instrument ou pratiqué le chant ?"],["solfege","Avez-vous suivi des cours de solfège ?"],["years","Avez-vous pratiqué la musique pendant au moins trois ans ?"],["frenchNative","Le français est-il votre langue maternelle ?"],["tonalLang","Parlez-vous une langue à tons ?"],["hearingOK","Pensez-vous avoir une audition normale ?"]]) {
    show(q + "\n\nO = Oui    N = Non"); profile[key] = (await waitKey(["O","N"])) === "O" ? "oui" : "non";
  }
  logRow({ phase: "questionnaire", trial: 0, profile });
  await instruction("REGLAGE DU VOLUME\n\nAppuyez sur ESPACE pour écouter un son de test, puis réglez le volume.\nAppuyez sur ENTRÉE pour continuer.");
  let volumeDone = false; while (!volumeDone) { const k = await waitKey([" ","ENTER"]); if (k === " ") playTone(440, 800); else volumeDone = true; }
  const practice = shuffle([1,1,3,3,5,5].map((i, n) => ({ f1: window.contFreqs[i-1], f2: n % 2 ? window.contFreqs[i] : window.contFreqs[i-1], same: !(n % 2) })));
  await instruction(`ENTRAINEMENT\n\nTouche ${sameKey} = identique\nTouche ${diffKey} = différente\n\nRépondez après le deuxième son.`);
  await runPairPhase("practice", practice, tone1Dur, 1500);
  const e1 = [];
  noteFreqs.forEach((f, i) => { for (let n=0;n<7;n++) e1.push({f1:f,f2:f,l1:noteNames[i],l2:noteNames[i],same:true}); });
  for (let i=0;i<8;i++) for (let j=i+1;j<8;j++) e1.push({f1:noteFreqs[i],f2:noteFreqs[j],l1:noteNames[i],l2:noteNames[j],semis:noteSemis[j]-noteSemis[i],degree:j-i+1,same:false});
  await instruction(`EXPERIENCE 1 : NOTES IDENTIQUES OU DIFFERENTES\n\nTouche ${sameKey} = identique\nTouche ${diffKey} = différente\n\nRépondez vite et précisément.`);
  await runPairPhase("exp1_discrimination", shuffle(e1).slice(0, Math.floor(e1.length/2)), tone1Dur, 3000);
  const e2 = []; [1,3,5,7,9].forEach(i => { for(let n=0;n<8;n++) e2.push({i1:i,i2:i,same:true}); });
  for(let i=1;i<=7;i+=2) for(let n=0;n<5;n++) { e2.push({i1:i,i2:i+2,same:false,pairType:`D${i}${i+2}`},{i1:i+2,i2:i,same:false,pairType:`D${i}${i+2}`}); }
  await instruction(`EXPERIENCE 2 : SONS PROCHES\n\nMême consigne : ${sameKey} = identique, ${diffKey} = différente.`);
  await runPairPhase("exp2_discrimination", shuffle(e2).slice(0, Math.floor(e2.length/2)).map(t => ({...t,f1:window.contFreqs[t.i1-1],f2:window.contFreqs[t.i2-1],stim1:t.i1,stim2:t.i2})), tone2Dur, 3000);
  const ids = shuffle(Array.from({length:9},(_,i)=>i+1).flatMap(i=>Array(10).fill(i))).slice(0,45);
  await instruction(`IDENTIFICATION DES NOTES\n\nTouche ${lowKey} = ${lowLabel}\nTouche ${highKey} = ${highLabel}\n\nRépondez avec votre première impression.`);
  for (let i=0;i<ids.length;i++) { progress(`exp2_identification — essai ${i+1}/${ids.length}`); const idx=ids[i], r=await singleTrial(window.contFreqs[idx-1],[lowKey,highKey],5000); logRow({phase:"exp2_identification",trial:i+1,stim1:idx,f1:window.contFreqs[idx-1],response_key:r.key||"none",response:r.key===lowKey?lowLabel:r.key===highKey?highLabel:"aucune_reponse",rt_ms:r.rt,correct:-1,rating_1to5:-1}); }
  const e3=[]; [1,2,3,4,5,7,12].forEach(n=>{const ratio=2**(n/12); e3.push({f1:262.29,f2:262.29*ratio,semis:n,dir:"up"},{f1:330.87,f2:330.87*ratio,semis:n,dir:"up"},{f1:524.42,f2:524.42/ratio,semis:n,dir:"down"});});
  await instruction("EXPERIENCE 3 : DISTANCE ENTRE LES NOTES\n\nD=1 très proches, F=2 plutôt proches, G=3 moyennement éloignées, H=4 plutôt éloignées, J=5 très éloignées.");
  for (let i=0;i<e3.length;i++) { progress(`exp3_distance — essai ${i+1}/${e3.length}`); await sleep(500); playTone(e3[i].f1); await sleep(tone1Dur+gapDur); playTone(e3[i].f2); await sleep(tone1Dur); const started=performance.now(), key=await waitKey(["D","F","G","H","J"]); logRow({phase:"exp3_distance",trial:i+1,f1:e3[i].f1,f2:e3[i].f2,semitones:e3[i].semis,pair_type:e3[i].dir,response_key:key,response:key,rt_ms:performance.now()-started,correct:-1,rating_1to5:["D","F","G","H","J"].indexOf(key)+1}); }
  if (!window.GOOGLE_SHEETS_ENDPOINT) throw new Error("L'URL Google Sheets n'est pas configurée.");
  await fetch(window.GOOGLE_SHEETS_ENDPOINT, {
    method: "POST",
    mode: "no-cors",
    headers: {"Content-Type": "text/plain;charset=utf-8"},
    body: JSON.stringify({session: sessionId, profile, rows})
  });
  show("MERCI POUR VOTRE PARTICIPATION !\n\nVos réponses ont été enregistrées.");
}
show("Cette expérience utilise le son. Utilisez un casque ou des écouteurs.", [["Commencer", () => main().catch(e => show(`Erreur : ${e.message}`))]]);
