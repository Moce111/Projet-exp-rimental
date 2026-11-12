// Si le script est lié au Google Sheet (Extensions > Apps Script), laissez vide.
// Si le script a été créé à part sur script.google.com, collez ici l'ID du Sheet.
const SPREADSHEET_ID = "";

const HEADERS = [
  "received_at", "session", "participant", "phase", "trial", "continuum",
  "response_key", "response", "rt_ms", "correct", "rating_1to5",
  "payload_json"
];

function doPost(event) {
  const lock = LockService.getScriptLock();
  try {
    const submission = JSON.parse(event.postData.contents);
    if (!submission.session || !Array.isArray(submission.rows) || submission.rows.length === 0) {
      return jsonResponse({ok: false, error: "invalid submission"});
    }

    lock.waitLock(20000); // évite que deux envois simultanés s'écrasent
    const spreadsheet = SPREADSHEET_ID
      ? SpreadsheetApp.openById(SPREADSHEET_ID)
      : SpreadsheetApp.getActiveSpreadsheet();
    const sheet = spreadsheet.getSheets()[0];
    if (sheet.getLastRow() === 0) {
      sheet.appendRow(HEADERS);
    }

    const receivedAt = new Date().toISOString();
    const values = submission.rows.map(row => [
      receivedAt,
      submission.session,
      value(row, "participant") || submission.participant || "",
      value(row, "phase"),
      value(row, "trial"),
      value(row, "continuum"),
      value(row, "response_key"),
      value(row, "response"),
      value(row, "rt_ms"),
      value(row, "correct"),
      value(row, "rating_1to5"),
      JSON.stringify(row)
    ]);
    sheet.getRange(sheet.getLastRow() + 1, 1, values.length, HEADERS.length).setValues(values);
    return jsonResponse({ok: true});
  } catch (error) {
    console.error(error);
    return jsonResponse({ok: false, error: "server error"});
  } finally {
    lock.releaseLock();
  }
}

function value(object, key) {
  return object[key] === undefined || object[key] === null ? "" : object[key];
}

function jsonResponse(body) {
  return ContentService.createTextOutput(JSON.stringify(body))
    .setMimeType(ContentService.MimeType.JSON);
}
