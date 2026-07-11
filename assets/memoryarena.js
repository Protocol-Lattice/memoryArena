const installCommand = 'go get github.com/Protocol-Lattice/memoryArena';
const copyButton = document.getElementById('copy-button');
const header = document.getElementById('site-header');
const mobileToggle = document.getElementById('mobile-toggle');
const navLinks = document.getElementById('nav-links');
const memoryGrid = document.getElementById('memory-grid');
const meter = document.getElementById('arena-meter');
const usedLabel = document.getElementById('used-label');
const remainingLabel = document.getElementById('remaining-label');
const allocateButton = document.getElementById('allocate-button');
const resetButton = document.getElementById('reset-button');
const capacity = 32;
let used = 18;

for (let index = 0; index < capacity; index += 1) {
  const cell = document.createElement('span');
  cell.className = 'memory-cell';
  memoryGrid.appendChild(cell);
}

function renderArena() {
  [...memoryGrid.children].forEach((cell, index) => {
    cell.classList.toggle('active', index < used);
  });
  meter.style.width = `${(used / capacity) * 100}%`;
  usedLabel.textContent = `Used ${used}`;
  remainingLabel.textContent = `Remaining ${capacity - used}`;
}

copyButton.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(installCommand);
    copyButton.textContent = 'Copied';
  } catch {
    copyButton.textContent = 'Select text';
  }
  window.setTimeout(() => { copyButton.textContent = 'Copy'; }, 1600);
});

allocateButton.addEventListener('click', () => {
  used = used >= capacity ? capacity : Math.min(capacity, used + Math.ceil(Math.random() * 4));
  renderArena();
});

resetButton.addEventListener('click', () => {
  used = 0;
  renderArena();
});

mobileToggle.addEventListener('click', () => {
  const open = navLinks.classList.toggle('open');
  mobileToggle.setAttribute('aria-expanded', String(open));
  mobileToggle.textContent = open ? '×' : '☰';
});

navLinks.querySelectorAll('a').forEach((link) => {
  link.addEventListener('click', () => {
    navLinks.classList.remove('open');
    mobileToggle.setAttribute('aria-expanded', 'false');
    mobileToggle.textContent = '☰';
  });
});

window.addEventListener('scroll', () => {
  header.classList.toggle('scrolled', window.scrollY > 12);
}, { passive: true });

renderArena();
