# Expérience de perception musicale

La version web est un site statique dans [web](./web). Elle peut être hébergée
par GitHub Pages. Les réponses sont ajoutées à un Google Sheet par
[Google Apps Script](./google-apps-script/Code.gs).

## Configurer Google Sheets

1. Créez un Google Sheet dans le Drive qui recevra les résultats.
2. Dans **Extensions > Apps Script**, copiez le contenu de
   `google-apps-script/Code.gs`.
3. Enregistrez, puis choisissez **Déployer > Nouveau déploiement**.
4. Choisissez **Application web**, exécutez l'application en tant que vous-même
   et autorisez l'accès à **Toute personne**.
5. Copiez l'URL `/exec` du déploiement dans
   `web/config.js`, à la place de la chaîne vide :

   ```js
   window.GOOGLE_SHEETS_ENDPOINT = "https://script.google.com/macros/s/ID/exec";
   ```

Ne mettez jamais de clé privée ou de mot de passe dans le dépôt GitHub. L'URL
du déploiement est publique ; ne placez donc pas d'informations confidentielles
dans les réponses et envisagez une protection supplémentaire si nécessaire.

## Publier sur GitHub Pages

1. Poussez le contenu du projet sur un dépôt GitHub.
2. Dans **Settings > Pages**, choisissez **GitHub Actions** comme source.
3. Poussez sur la branche `main`. Le workflow
   `.github/workflows/pages.yml` publiera automatiquement le dossier `web`.

L'URL sera généralement `https://UTILISATEUR.github.io/NOM-DU-DEPOT/`.
Le site utilise des chemins relatifs afin de fonctionner avec cette URL de
projet.

## Tester localement

Ouvrez `web/index.html` via un serveur HTTP local après avoir configuré
`web/config.js`. Par exemple :

```powershell
python -m http.server 8000 --directory .\web
```

Puis ouvrez <http://localhost:8000>.

Les réponses peuvent contenir des données d'étude : ajoutez l'information et
le consentement requis par votre protocole avant une collecte réelle.
