const puppeteer = require('B:/workgit/mqttpulse/node_modules/puppeteer');

async function test() {
  console.log('Puppeteer imported successfully:', typeof puppeteer.launch);
  const browser = await puppeteer.launch({
    headless: true,
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  console.log('Browser launched successfully!');
  await browser.close();
  console.log('Browser closed cleanly.');
}

test().catch(err => {
  console.error('Puppeteer test failed:', err);
  process.exit(1);
});
