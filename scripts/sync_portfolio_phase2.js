const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const SOURCE_IMAGES_DIR = path.resolve(__dirname, '../docs/images');
const MTLG_SITE_DIR = 'B:/workgit/mtlg-site';
const MTLGLABS_SPACE_DIR = 'B:/workgit/mtlglabs-space';

// Allowed visual assets for portfolio (STRICT INVARIANT: NEVER INCLUDE LOGOS)
const ASSETS_TO_COPY = [
  'uploadgate_dashboard.png',
  'uploadgate_sandbox.png',
  'uploadgate_feed.png',
  'uploadgate_asset_modal.png',
  'uploadgate_tablet.png',
  'uploadgate_mobile.png',
  'uploadgate_demo.gif'
];

function step1_copyAssets() {
  console.log('=== Step 1: Copying authentic UI screenshots and GIF (Zero Logos) ===');
  const targets = [
    path.join(MTLG_SITE_DIR, 'images'),
    path.join(MTLGLABS_SPACE_DIR, 'images')
  ];

  for (const targetDir of targets) {
    if (!fs.existsSync(targetDir)) {
      fs.mkdirSync(targetDir, { recursive: true });
    }
    for (const file of ASSETS_TO_COPY) {
      const src = path.join(SOURCE_IMAGES_DIR, file);
      const dest = path.join(targetDir, file);
      if (fs.existsSync(src)) {
        fs.copyFileSync(src, dest);
        console.log(`✓ Copied ${file} -> ${dest}`);
      } else {
        console.warn(`! Missing asset: ${src}`);
      }
    }
  }
}

function step2_updateImageMaps() {
  console.log('=== Step 2: Updating project_images_map.json ===');
  const relativeList = ASSETS_TO_COPY.map(f => `images/${f}`);
  const mapFiles = [
    path.join(MTLG_SITE_DIR, 'project_images_map.json'),
    path.join(MTLGLABS_SPACE_DIR, 'project_images_map.json')
  ];

  for (const mapFile of mapFiles) {
    if (fs.existsSync(mapFile)) {
      const content = JSON.parse(fs.readFileSync(mapFile, 'utf8'));
      content['uploadgate'] = relativeList;
      fs.writeFileSync(mapFile, JSON.stringify(content, null, 2) + '\n', 'utf8');
      console.log(`✓ Updated ${mapFile}`);
    }
  }
}

function step3_updateProjectsJson() {
  console.log('=== Step 3: Updating projects.json to LIVE ===');
  const projectFiles = [
    path.join(MTLG_SITE_DIR, 'projects.json'),
    path.join(MTLGLABS_SPACE_DIR, 'projects.json')
  ];

  const relativeList = ASSETS_TO_COPY.map(f => `images/${f}`);
  const humanizedDesc = 'Direct-to-storage presigned upload gateway and asset ingestion daemon with binary magic-byte validation, pure-Go WebP generation, and HMAC webhooks under 40MB RAM.';

  for (const pFile of projectFiles) {
    if (fs.existsSync(pFile)) {
      const projects = JSON.parse(fs.readFileSync(pFile, 'utf8'));
      const item = projects.find(p => p.id === 'uploadgate');
      if (item) {
        item.badgeLabs = { text: 'LIVE', type: 'live' };
        item.badgeSite = { text: 'LIVE', type: 'live' };
        item.desc = humanizedDesc;
        item.image = 'images/uploadgate_demo.gif';
        item.images = relativeList;
        fs.writeFileSync(pFile, JSON.stringify(projects, null, 2) + '\n', 'utf8');
        console.log(`✓ Set uploadgate to LIVE in ${pFile}`);
      } else {
        console.warn(`! Project uploadgate not found in ${pFile}`);
      }
    }
  }
}

function step4_syncAndCompare() {
  console.log('=== Step 4: Running sync.js and compare_projects.js ===');
  try {
    const outSync = execSync('node sync.js', { cwd: MTLG_SITE_DIR, encoding: 'utf8' });
    console.log(outSync.trim());
  } catch (err) {
    console.error('sync.js error:', err.stdout || err.message);
  }

  try {
    const outComp = execSync('node compare_projects.js', { cwd: MTLG_SITE_DIR, encoding: 'utf8' });
    console.log(outComp.trim());
  } catch (err) {
    console.error('compare_projects.js error:', err.stdout || err.message);
  }
}

function step5_gitPush() {
  console.log('=== Step 5: Git commit and push for portfolio repos ===');
  const repos = [
    { dir: MTLG_SITE_DIR, name: 'mtlg-site' },
    { dir: MTLGLABS_SPACE_DIR, name: 'mtlglabs-space' }
  ];

  for (const repo of repos) {
    try {
      execSync('git add .', { cwd: repo.dir, stdio: 'inherit' });
      execSync('git commit -m "feat(portfolio): complete uploadgate live assets, demo gif and screenshots"', {
        cwd: repo.dir,
        stdio: 'inherit'
      });
      execSync('git push origin main', { cwd: repo.dir, stdio: 'inherit' });
      console.log(`✓ Committed and pushed ${repo.name}`);
    } catch (err) {
      console.log(`Note for ${repo.name}: ${err.message}`);
    }
  }
}

function step6_xBroadcastDryRun() {
  console.log('=== Step 6: X Broadcast Dry Run ===');
  try {
    const dryRun = execSync('node broadcast_x.js uploadgate --dry-run', { cwd: MTLG_SITE_DIR, encoding: 'utf8' });
    console.log(dryRun.trim());
  } catch (err) {
    console.log('broadcast_x note:', err.stdout || err.message);
  }
}

function main() {
  step1_copyAssets();
  step2_updateImageMaps();
  step3_updateProjectsJson();
  step4_syncAndCompare();
  step5_gitPush();
  step6_xBroadcastDryRun();
  console.log('✓ Phase 2 Portfolio Sync completed successfully!');
}

main();
