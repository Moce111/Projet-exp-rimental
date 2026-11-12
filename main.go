// Perception musicale chez des auditeurs francophones.
//
// Réplication adaptée de Liang et al. (2025), Behavioral Sciences 15(1), 30
// (doi:10.3390/bs15010030), avec une extension (expérience 3 : distance subjective).
//
// Écarts par rapport à l'article (voir aussi le message d'accompagnement) :
//   - timbre synthétique (5 harmoniques + decroissance) au lieu d'un piano echantillonne ;
//   - expérience 2 : sons de 600 ms (1500 ms dans l'article) et moins de répétitions,
//     afin que la session totale dure ~10 min au lieu de ~60 min ;
//   - un seul continuum par participant (C4-D4 ou A4-B4), comme dans l'article,
//     attribué selon la parité du numéro de sujet ;
//   - touches F/J contrebalancées entre participants, comme dans l'article.
package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	toneLevel  = 0.08 // niveau RMS des sons (le volume se règle sur l'ordinateur)
	tone1Dur   = 600  // ms : expériences 1 et 3 (comme l'article)
	tone2Dur   = 600  // ms : expérience 2 (article : 1500 ms, raccourci)
	gapDur     = 500  // ms entre les deux sons (comme l'article)
	fixDur     = 500  // ms de croix de fixation (comme l'article)
	respWindow = 1500 // ms de fenêtre de réponse pendant l'entraînement
	expWindow  = 3000 // ms de fenêtre de réponse pendant les expériences 1 et 2
	idWindow   = 5000 // ms de fenêtre de réponse pour l'identification
)

// ---------------------------------------------------------------------------
// Stimuli
// ---------------------------------------------------------------------------

// synthTone produit un son harmonique de type "instrument à cordes frappées" :
// 5 harmoniques, décroissance exponentielle douce, rampes de 15 ms,
// et normalisation RMS pour que toutes les hauteurs aient la même sonie physique.
func synthTone(freq float64, durMs int) *stimuli.Tone {
	sr := 44100.0
	n := int(sr) * durMs / 1000
	weights := []float64{1.0, 0.55, 0.30, 0.18, 0.10}
	rampN := int(0.015 * sr)

	buf := make([]float64, n)
	var sumSq float64
	for i := range buf {
		t := float64(i) / sr
		var v float64
		for h, w := range weights {
			v += w * math.Sin(2*math.Pi*freq*float64(h+1)*t)
		}
		v *= math.Exp(-t / 1.2)
		if i < rampN {
			v *= float64(i) / float64(rampN)
		} else if i >= n-rampN {
			v *= float64(n-1-i) / float64(rampN)
		}
		buf[i] = v
		sumSq += v * v
	}
	gain := toneLevel / math.Sqrt(sumSq/float64(n))

	data := make([]byte, n*4) // float32 little-endian, mono
	for i, v := range buf {
		bits := math.Float32bits(float32(v * gain))
		data[i*4] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}
	return &stimuli.Tone{
		Frequency: freq,
		Duration:  durMs,
		Amplitude: toneLevel,
		Data:      data,
	}
}

// Fréquences de l'article (Exp. 1) : C4 D4 E4 F4 G4 A4 B4 C5.
var (
	noteNames = []string{"C4", "D4", "E4", "F4", "G4", "A4", "B4", "C5"}
	noteFreqs = []float64{262.29, 294.63, 330.87, 350.54, 392.98, 441.51, 495.55, 524.42}
	noteSemis = []int{0, 2, 4, 5, 7, 9, 11, 12}

	// Notes d'entraînement de l'article : F#4, G#4, A#4.
	practiceFreqs = []float64{371.27, 416.72, 467.77}

	// Continua à 9 pas (Tableau 3 de l'article).
	contCD = []float64{262.29, 266.35, 270.27, 274.17, 278.23, 282.36, 286.42, 290.49, 294.63}
	contAB = []float64{441.51, 448.17, 454.93, 461.60, 468.35, 475.18, 481.87, 488.69, 495.55}
)

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

type keymap struct {
	same, diff, low, high                 control.Keycode
	sameName, diffName, lowName, highName string
}

type profile struct {
	instrument, solfege, years, frenchNative, tonalLang, hearingOK string
}

type session struct {
	exp        *control.Experiment
	rng        *rand.Rand
	tones      map[string]*stimuli.Tone
	km         keymap
	keyMapping string
	continuum  string
	contFreqs  []float64
	lowLabel   string // solfège de la note grave du continuum
	highLabel  string // solfège de la note aiguë
	prof       profile
	fix        *stimuli.TextLine
	stats      map[string]*phaseStats
}

type phaseStats struct {
	total, answered, scored, correct int
}

type row struct {
	phase     string
	trial     int
	continuum string
	stim1     string
	stim2     string
	f1, f2    float64
	semitones int
	degree    int
	pairType  string
	key, resp string
	rt        float64
	correct   int
	rating    int
}

type result struct {
	key      control.Keycode
	rt       float64 // ms depuis le début du 2e son (ou du son unique)
	answered bool
}

func (s *session) log(r row) {
	s.exp.Data.Add(
		s.exp.SubjectID, r.phase, r.trial, r.continuum, r.stim1, r.stim2,
		r.f1, r.f2, r.semitones, r.degree, r.pairType, s.keyMapping,
		r.key, r.resp, r.rt, r.correct, r.rating,
		s.prof.instrument, s.prof.solfege, s.prof.years,
		s.prof.frenchNative, s.prof.tonalLang, s.prof.hearingOK,
	)
	stats := s.stats[r.phase]
	stats.total++
	if r.key != "none" {
		stats.answered++
	}
	if r.correct == 1 {
		stats.correct++
	}
	if r.correct >= 0 {
		stats.scored++
	}
}

func (s *session) tone(freq float64, dur int) (*stimuli.Tone, error) {
	key := fmt.Sprintf("%.3f_%d", freq, dur)
	if t, ok := s.tones[key]; ok {
		return t, nil
	}
	t := synthTone(freq, dur)
	if err := t.PreloadDevice(s.exp.AudioDevice); err != nil {
		return nil, err
	}
	s.tones[key] = t
	return t, nil
}

func stopTone(t *stimuli.Tone) {
	if t != nil && t.Stream != nil {
		_ = t.Stream.Clear()
	}
}

func shuffle[T any](r *rand.Rand, list []T) {
	r.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
}

// assign attribue continuum et mapping de touches.
// Numéro de sujet impair -> C4-D4, pair -> A4-B4 ; (id/2) pair -> F=identique.
// Sans numéro de sujet (0), tirage aléatoire.
func (s *session) assign() {
	id := s.exp.SubjectID
	var cd, swap bool
	if id > 0 {
		cd = id%2 == 1
		swap = (id/2)%2 == 1
	} else {
		cd = s.rng.Intn(2) == 0
		swap = s.rng.Intn(2) == 1
	}

	if cd {
		s.continuum, s.contFreqs, s.lowLabel, s.highLabel = "C4-D4", contCD, "do", "ré"
	} else {
		s.continuum, s.contFreqs, s.lowLabel, s.highLabel = "A4-B4", contAB, "la", "si"
	}

	if !swap {
		s.km = keymap{control.K_F, control.K_J, control.K_F, control.K_J, "F", "J", "F", "J"}
		s.keyMapping = "F=same/low;J=diff/high"
	} else {
		s.km = keymap{control.K_J, control.K_F, control.K_J, control.K_F, "J", "F", "J", "F"}
		s.keyMapping = "J=same/low;F=diff/high"
	}
}

func (s *session) sameDiffRule() string {
	return fmt.Sprintf("Touche %s = les deux notes sont IDENTIQUES\nTouche %s = les deux notes sont DIFFERENTES",
		s.km.sameName, s.km.diffName)
}

func (s *session) fixation() error {
	return s.exp.ShowTimed(s.fix, fixDur)
}

func (s *session) notice(msg string, ms int) error {
	tl := stimuli.NewTextLine(msg, 0, 0, control.White)
	defer tl.Unload()
	return s.exp.ShowTimed(tl, ms)
}

// ---------------------------------------------------------------------------
// Essais élémentaires
// ---------------------------------------------------------------------------

// pairTrial : croix, son 1, silence, son 2, réponse. RT mesuré depuis le début du son 2.
func (s *session) pairTrial(f1, f2 float64, dur int, keys []control.Keycode, timeout int) (result, error) {
	var res result
	t1, err := s.tone(f1, dur)
	if err != nil {
		return res, err
	}
	t2, err := s.tone(f2, dur)
	if err != nil {
		return res, err
	}
	if err := s.fixation(); err != nil {
		return res, err
	}
	if err := t1.Play(); err != nil {
		return res, err
	}
	if err := s.exp.Wait(dur + gapDur); err != nil {
		return res, err
	}
	onset, err := t2.PlayTS()
	if err != nil {
		return res, err
	}
	key, ev, err := s.exp.Keyboard.GetKeyEventTS(keys, timeout)
	if err != nil {
		return res, err
	}
	stopTone(t2)
	if ev != 0 {
		res = result{key: key, rt: float64(int64(ev)-int64(onset)) / 1e6, answered: true}
	} else {
		res.rt = -1
	}
	return res, nil
}

// singleTrial : croix, un son, réponse (identification).
func (s *session) singleTrial(f float64, dur int, keys []control.Keycode, timeout int) (result, error) {
	var res result
	t, err := s.tone(f, dur)
	if err != nil {
		return res, err
	}
	if err := s.fixation(); err != nil {
		return res, err
	}
	onset, err := t.PlayTS()
	if err != nil {
		return res, err
	}
	key, ev, err := s.exp.Keyboard.GetKeyEventTS(keys, timeout)
	if err != nil {
		return res, err
	}
	stopTone(t)
	if ev != 0 {
		res = result{key: key, rt: float64(int64(ev)-int64(onset)) / 1e6, answered: true}
	} else {
		res.rt = -1
	}
	return res, nil
}

// scoreSD traduit une réponse identique/différent.
func (s *session) scoreSD(r result, same bool) (keyName, resp string, correct int) {
	if !r.answered {
		return "none", "aucune_reponse", -1
	}
	if r.key == s.km.same {
		if same {
			correct = 1
		}
		return s.km.sameName, "identique", correct
	}
	if !same {
		correct = 1
	}
	return s.km.diffName, "different", correct
}

func (s *session) sdKeys() []control.Keycode {
	return []control.Keycode{s.km.same, s.km.diff}
}

func (s *session) pause(reminder string) error {
	return s.exp.ShowInstructions(
		"Vous pouvez faire une courte pause.\n\n" + reminder + "\n\nAppuyez sur ESPACE pour reprendre.")
}

// ---------------------------------------------------------------------------
// Phases
// ---------------------------------------------------------------------------

func (s *session) askYesNo(q string) (string, error) {
	box := s.exp.FittedTextBox(q + "\n\nO = Oui\nN = Non")
	defer box.Unload()
	if err := s.exp.Show(box); err != nil {
		return "", err
	}
	key, _, err := s.exp.Keyboard.WaitKeysRT([]control.Keycode{control.K_O, control.K_N}, -1)
	if err != nil {
		return "", err
	}
	if key == control.K_O {
		return "oui", nil
	}
	return "non", nil
}

func (s *session) questionnaire() error {
	if err := s.exp.ShowInstructions(
		"QUESTIONNAIRE PRELIMINAIRE\n\n" +
			"Nous souhaitons connaître votre expérience musicale et linguistique.\n\n" +
			"Répondez avec les touches O (oui) et N (non).\n\n" +
			"Appuyez sur ESPACE pour commencer."); err != nil {
		return err
	}
	qs := []struct {
		dst *string
		q   string
	}{
		{&s.prof.instrument, "Avez-vous déjà joué d'un instrument de musique ou pratiqué le chant ?"},
		{&s.prof.solfege, "Avez-vous déjà suivi des cours de solfège ou appris à lire une partition ?"},
		{&s.prof.years, "Avez-vous pratiqué la musique pendant au moins trois ans ?"},
		{&s.prof.frenchNative, "Le français est-il votre langue maternelle ?"},
		{&s.prof.tonalLang, "Parlez-vous ou avez-vous pratiqué une langue à tons (mandarin, cantonais, vietnamien, thaï...) ?"},
		{&s.prof.hearingOK, "Pensez-vous avoir une audition normale ?"},
	}
	for _, q := range qs {
		a, err := s.askYesNo(q.q)
		if err != nil {
			return err
		}
		*q.dst = a
	}
	s.log(row{phase: "questionnaire", trial: 0, semitones: -1, degree: -1, rt: -1, correct: -1, rating: -1})
	return nil
}

func (s *session) volumeCheck() error {
	box := s.exp.FittedTextBox(
		"REGLAGE DU VOLUME\n\n" +
			"Portez votre casque.\n\n" +
			"ESPACE = écouter un son de test (autant de fois que nécessaire)\n" +
			"Réglez le volume de l'ordinateur à un niveau confortable.\n\n" +
			"ENTREE = continuer")
	defer box.Unload()
	if err := s.exp.Show(box); err != nil {
		return err
	}
	t, err := s.tone(440, 800)
	if err != nil {
		return err
	}
	for {
		k, err := s.exp.Keyboard.WaitKeys([]control.Keycode{control.K_SPACE, control.K_RETURN}, -1)
		if err != nil {
			return err
		}
		if k == control.K_RETURN {
			return nil
		}
		if err := t.Play(); err != nil {
			return err
		}
	}
}

type pairSpec struct {
	f1, f2        float64
	l1, l2        string
	semis, degree int
	same          bool
}

func (s *session) practice() error {
	if err := s.exp.ShowInstructions(
		"ENTRAINEMENT\n\n" +
			"Vous allez entendre deux notes successives.\n\n" +
			s.sameDiffRule() + "\n\n" +
			"Vous allez faire 6 essais d'entraînement sur ce registre de notes.\n" +
			"Répondez dès que vous avez entendu le deuxième son.\n" +
			"Pendant l'entraînement seulement, vous saurez si votre réponse est correcte.\n\n" +
			"Appuyez sur ESPACE pour commencer."); err != nil {
		return err
	}

	var list []pairSpec
	for _, idx := range []int{1, 3, 5} {
		f := s.contFreqs[idx-1]
		list = append(list, pairSpec{f1: f, f2: f, same: true})
		if idx < 5 {
			list = append(list, pairSpec{f1: f, f2: s.contFreqs[idx], same: false})
		}
	}
	shuffle(s.rng, list)

	for i, p := range list {
		r, err := s.pairTrial(p.f1, p.f2, tone1Dur, s.sdKeys(), respWindow)
		if err != nil {
			return err
		}
		k, resp, c := s.scoreSD(r, p.same)
		s.log(row{phase: "practice", trial: i + 1, f1: p.f1, f2: p.f2, semitones: -1, degree: -1,
			pairType: map[bool]string{true: "same", false: "diff"}[p.same],
			key:      k, resp: resp, rt: r.rt, correct: c, rating: -1})
		msg := "Incorrect"
		switch c {
		case 1:
			msg = "Correct"
		case -1:
			msg = "Pas de réponse détectée"
		}
		if err := s.notice(msg, 600); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) exp1() error {
	if err := s.exp.ShowInstructions(
		"EXPERIENCE 1 : NOTES IDENTIQUES OU DIFFERENTES\n\n" +
			"Vous allez entendre deux notes successives.\n" +
			"Votre tâche est de dire si elles sont identiques ou différentes.\n\n" +
			s.sameDiffRule() + "\n\n" +
			"Répondez le plus vite et le plus précisément possible.\n" +
			"Il n'y a plus d'indication de réussite.\n\n" +
			"Appuyez sur ESPACE pour commencer."); err != nil {
		return err
	}

	var trials []pairSpec
	// Génération complète, puis sélection aléatoire de la moitié des essais.
	for i := range noteFreqs {
		for rep := 0; rep < 7; rep++ {
			trials = append(trials, pairSpec{noteFreqs[i], noteFreqs[i], noteNames[i], noteNames[i], 0, 1, true})
		}
	}
	for i := 0; i < len(noteFreqs); i++ {
		for j := i + 1; j < len(noteFreqs); j++ {
			semis := noteSemis[j] - noteSemis[i]
			deg := j - i + 1 // 2e, 3e ... 8e (degrés de l'article)
			trials = append(trials,
				pairSpec{noteFreqs[i], noteFreqs[j], noteNames[i], noteNames[j], semis, deg, false},
				pairSpec{noteFreqs[j], noteFreqs[i], noteNames[j], noteNames[i], semis, deg, false})
		}
	}
	shuffle(s.rng, trials)
	trials = trials[:len(trials)/2]

	for i, p := range trials {
		if i == len(trials)/2 {
			if err := s.pause(s.sameDiffRule()); err != nil {
				return err
			}
		}
		r, err := s.pairTrial(p.f1, p.f2, tone1Dur, s.sdKeys(), expWindow)
		if err != nil {
			return err
		}
		k, resp, c := s.scoreSD(r, p.same)
		s.log(row{phase: "exp1_discrimination", trial: i + 1, stim1: p.l1, stim2: p.l2,
			f1: p.f1, f2: p.f2, semitones: p.semis, degree: p.degree,
			pairType: map[bool]string{true: "same", false: "diff"}[p.same],
			key:      k, resp: resp, rt: r.rt, correct: c, rating: -1})
		if !r.answered {
			if err := s.notice("Pas de réponse détectée", 400); err != nil {
				return err
			}
		} else if err := s.notice("Réponse prise en compte", 250); err != nil {
			return err
		}
	}
	return nil
}

type contSpec struct{ i1, i2 int }

func (s *session) exp2() error {
	// ---- Discrimination sur le continuum ----
	if err := s.exp.ShowInstructions(
		"EXPERIENCE 2 : SONS PROCHES\n\n" +
			"Les sons seront cette fois très proches les uns des autres.\n" +
			"Même consigne : les deux notes sont-elles identiques ou différentes ?\n\n" +
			s.sameDiffRule() + "\n\n" +
			"Certaines paires seront difficiles : fiez-vous à votre impression.\n\n" +
			"Appuyez sur ESPACE pour commencer."); err != nil {
		return err
	}

	var trials []contSpec
	// Génération complète, puis sélection aléatoire de la moitié des essais.
	for _, idx := range []int{1, 3, 5, 7, 9} {
		for rep := 0; rep < 8; rep++ {
			trials = append(trials, contSpec{idx, idx})
		}
	}
	for i := 1; i <= 7; i += 2 {
		for rep := 0; rep < 5; rep++ {
			trials = append(trials, contSpec{i, i + 2}, contSpec{i + 2, i})
		}
	}
	shuffle(s.rng, trials)
	trials = trials[:len(trials)/2]

	for i, tr := range trials {
		if i == len(trials)/2 {
			if err := s.pause(s.sameDiffRule()); err != nil {
				return err
			}
		}
		f1, f2 := s.contFreqs[tr.i1-1], s.contFreqs[tr.i2-1]
		r, err := s.pairTrial(f1, f2, tone2Dur, s.sdKeys(), expWindow)
		if err != nil {
			return err
		}
		same := tr.i1 == tr.i2
		k, resp, c := s.scoreSD(r, same)
		pt := "same"
		if !same {
			lo, hi := tr.i1, tr.i2
			if lo > hi {
				lo, hi = hi, lo
			}
			pt = fmt.Sprintf("D%d%d", lo, hi)
		}
		s.log(row{phase: "exp2_discrimination", trial: i + 1, continuum: s.continuum,
			stim1: fmt.Sprint(tr.i1), stim2: fmt.Sprint(tr.i2), f1: f1, f2: f2,
			semitones: -1, degree: -1, pairType: pt,
			key: k, resp: resp, rt: r.rt, correct: c, rating: -1})
		if !r.answered {
			if err := s.notice("Pas de réponse détectée", 400); err != nil {
				return err
			}
		} else if err := s.notice("Réponse prise en compte", 250); err != nil {
			return err
		}
	}

	// ---- Identification (solfège) ----
	idKeys := []control.Keycode{s.km.low, s.km.high}
	if err := s.exp.ShowInstructions(
		"IDENTIFICATION DES NOTES\n\n" +
			"Vous allez entendre une seule note à la fois.\n" +
			fmt.Sprintf("Dites si elle vous semble être un « %s » ou un « %s ».\n\n", s.lowLabel, s.highLabel) +
			fmt.Sprintf("Touche %s = %s\nTouche %s = %s\n\n", s.km.lowName, s.lowLabel, s.km.highName, s.highLabel) +
			"Certains sons seront ambigus : répondez avec votre première impression.\n\n" +
			"Appuyez sur ESPACE pour commencer."); err != nil {
		return err
	}

	var ids []int
	for note := 1; note <= 9; note++ {
		for rep := 0; rep < 10; rep++ {
			ids = append(ids, note)
		}
	}
	shuffle(s.rng, ids)
	ids = ids[:len(ids)/2]

	for i, idx := range ids {
		f := s.contFreqs[idx-1]
		r, err := s.singleTrial(f, tone2Dur, idKeys, idWindow)
		if err != nil {
			return err
		}
		keyName, resp := "none", "aucune_reponse"
		if r.answered {
			if r.key == s.km.low {
				keyName, resp = s.km.lowName, s.lowLabel
			} else {
				keyName, resp = s.km.highName, s.highLabel
			}
		}
		s.log(row{phase: "exp2_identification", trial: i + 1, continuum: s.continuum,
			stim1: fmt.Sprint(idx), f1: f, semitones: -1, degree: -1, pairType: "single",
			key: keyName, resp: resp, rt: r.rt, correct: -1, rating: -1})
		if !r.answered {
			if err := s.notice("Pas de réponse détectée", 400); err != nil {
				return err
			}
		} else if err := s.notice("Réponse prise en compte", 250); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) exp3() error {
	if err := s.exp.ShowInstructions(
		"EXPERIENCE 3 : A QUELLE DISTANCE SONT LES NOTES ?\n\n" +
			"Vous allez entendre deux notes successives.\n\n" +
			"Cette fois, il n'y a ni bonne ni mauvaise réponse.\n" +
			"Nous voulons savoir à quel point ces deux notes vous semblent éloignées l'une de l'autre.\n\n" +
			"Fiez-vous à votre intuition, sans chercher à calculer.\n\n" +
			"Vous répondrez de 1 à 5 avec les touches :\n" +
			"D = 1 : très proches\nF = 2 : plutôt proches\nG = 3 : moyennement éloignées\n" +
			"H = 4 : plutôt éloignées\nJ = 5 : très éloignées\n\n" +
			"Appuyez sur ESPACE pour commencer."); err != nil {
		return err
	}

	type dtrial struct {
		f1, f2 float64
		semis  int
		dir    string
	}
	var trials []dtrial
	for _, n := range []int{1, 2, 3, 4, 5, 7, 12} {
		ratio := math.Pow(2, float64(n)/12.0)
		trials = append(trials,
			dtrial{262.29, 262.29 * ratio, n, "up"},   // depuis C4, montant
			dtrial{330.87, 330.87 * ratio, n, "up"},   // depuis E4, montant
			dtrial{524.42, 524.42 / ratio, n, "down"}, // depuis C5, descendant
		)
	}
	shuffle(s.rng, trials)

	prompt := s.exp.FittedTextBox(
		"A quel point ces deux notes vous semblent-elles éloignées ?\n\n" +
			"D = 1 : très proches\nF = 2 : plutôt proches\nG = 3 : moyennement éloignées\n" +
			"H = 4 : plutôt éloignées\nJ = 5 : très éloignées")
	defer prompt.Unload()

	keys := []control.Keycode{control.K_D, control.K_F, control.K_G, control.K_H, control.K_J}

	for i, d := range trials {
		t1, err := s.tone(d.f1, tone1Dur)
		if err != nil {
			return err
		}
		t2, err := s.tone(d.f2, tone1Dur)
		if err != nil {
			return err
		}
		if err := s.fixation(); err != nil {
			return err
		}
		if err := t1.Play(); err != nil {
			return err
		}
		if err := s.exp.Wait(tone1Dur + gapDur); err != nil {
			return err
		}
		if err := t2.Play(); err != nil {
			return err
		}
		if err := s.exp.Wait(tone1Dur); err != nil {
			return err
		}
		onset, err := s.exp.ShowTS(prompt)
		if err != nil {
			return err
		}
		key, ev, err := s.exp.Keyboard.GetKeyEventTS(keys, -1)
		if err != nil {
			return err
		}
		rating, rt, keyName := -1, -1.0, "none"
		if ev != 0 {
			for k, kc := range keys {
				if kc == key {
					rating = k + 1
				}
			}
			rt = float64(int64(ev)-int64(onset)) / 1e6
			keyName = string("DFGHJ"[rating-1])
		}
		s.log(row{phase: "exp3_distance", trial: i + 1, f1: d.f1, f2: d.f2,
			semitones: d.semis, degree: -1, pairType: d.dir,
			key: keyName, rt: rt, correct: -1, rating: rating})
	}
	return nil
}

func (s *session) finalStats() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determination du dossier des resultats: %w", err)
	}
	dataDir := filepath.Join(home, "goxpy_data")
	stats := func(phase, label string) string {
		p := s.stats[phase]
		accuracy := "n/a"
		if p.scored > 0 {
			accuracy = fmt.Sprintf("%.0f%%", 100*float64(p.correct)/float64(p.scored))
		}
		return fmt.Sprintf("%s : %d essais, %d réponses, réussite %s", label, p.total, p.answered, accuracy)
	}
	return fmt.Sprintf(
		"STATISTIQUES\n\n%s\n%s\n%s\n\nRésultats enregistrés dans :\n%s",
		stats("exp1_discrimination", "Partie 1"),
		stats("exp2_discrimination", "Partie 2 - discrimination"),
		stats("exp2_identification", "Partie 2 - identification"),
		dataDir,
	), nil
}

// ---------------------------------------------------------------------------

func main() {
	exp := control.NewExperimentFromFlags(
		"Perception_Musicale_Francais",
		control.Black,
		control.White,
		32,
	)
	defer exp.End()

	exp.AddDataVariableNames([]string{
		"subject", "phase", "trial", "continuum", "stim1", "stim2",
		"freq1_hz", "freq2_hz", "interval_semitones", "interval_degree", "pair_type", "key_mapping",
		"response_key", "response", "rt_ms", "correct", "rating_1to5",
		"instrument_voice", "solfege", "music_3plus_years",
		"french_native", "tonal_language", "hearing_ok",
	})

	s := &session{
		exp:   exp,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
		tones: make(map[string]*stimuli.Tone),
		fix:   stimuli.NewTextLine("+", 0, 0, control.White),
		stats: map[string]*phaseStats{
			"practice":            {},
			"exp1_discrimination": {},
			"exp2_discrimination": {},
			"exp2_identification": {},
		},
	}
	s.assign()
	defer func() {
		for _, t := range s.tones {
			if err := t.Unload(); err != nil {
				log.Printf("Erreur lors de la libération d'un son : %v", err)
			}
		}
	}()

	err := exp.Run(func() error {
		if err := exp.ShowInstructions(
			"EXPERIENCE SUR LA PERCEPTION DES NOTES DE MUSIQUE\n\n" +
				"Vous allez écouter des sons et répondre avec le clavier.\n" +
				"Durée totale : environ 10 minutes.\n\n" +
				"Installez-vous au calme et utilisez un casque ou des écouteurs.\n" +
				"Il n'est pas nécessaire de connaître la musique.\n" +
				"Répondez spontanément.\n\n" +
				"Vous pouvez interrompre l'expérience avec ECHAP.\n\n" +
				"Appuyez sur ESPACE pour continuer."); err != nil {
			return err
		}
		if err := s.questionnaire(); err != nil {
			return err
		}
		if err := s.volumeCheck(); err != nil {
			return err
		}
		if err := s.practice(); err != nil {
			return err
		}
		if err := s.exp1(); err != nil {
			return err
		}
		if err := s.exp2(); err != nil {
			return err
		}
		if err := s.exp3(); err != nil {
			return err
		}
		finalStats, err := s.finalStats()
		if err != nil {
			return err
		}
		if err := exp.ShowEndMessage(
			"MERCI POUR VOTRE PARTICIPATION !\n\n" +
				"Vous avez terminé toutes les tâches.\n" +
				"Vos réponses ont été enregistrées.\n\n" +
				finalStats + "\n\n" +
				"Appuyez sur une touche pour fermer."); err != nil {
			return err
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		log.Printf("Erreur pendant l'expérience : %v", err)
	}
}
