// Ezhik Ideas — Frontend App

const API_URL = window.location.origin;

// Telegram WebApp integration
const Telegram = window.Telegram.WebApp;
Telegram.expand();
Telegram.ready();

let currentIdea = '';
let stats = { count: 0 };

// DOM elements
const generateBtn = document.getElementById('generate-btn');
const categorySelect = document.getElementById('category');
const ideaBox = document.getElementById('idea-box');
const ideaText = document.getElementById('idea-text');
const actions = document.getElementById('actions');
const likeBtn = document.getElementById('like-btn');
const dislikeBtn = document.getElementById('dislike-btn');
const regenBtn = document.getElementById('regen-btn');
const statsCount = document.getElementById('stats-count');

// Event listeners
generateBtn.addEventListener('click', generateIdea);
regenBtn.addEventListener('click', generateIdea);
likeBtn.addEventListener('click', () => sendFeedback('like'));
dislikeBtn.addEventListener('click', () => sendFeedback('dislike'));

// Load stats on start
loadStats();

async function generateIdea() {
  const category = categorySelect.value;
  
  // UI state: loading
  generateBtn.disabled = true;
  generateBtn.textContent = 'Думаю... 💭';
  ideaBox.classList.remove('hidden');
  ideaText.classList.add('loading');
  ideaText.textContent = 'Ежик думает...';
  actions.classList.add('hidden');

  try {
    const response = await fetch(`${API_URL}/api/idea?category=${category}`);
    const data = await response.json();
    
    currentIdea = data.idea;
    ideaText.textContent = currentIdea;
    ideaText.classList.remove('loading');
    actions.classList.remove('hidden');
    
    // Update stats
    stats.count++;
    statsCount.textContent = stats.count;
    
    Telegram.MainButton.text = 'Поделиться';
    Telegram.MainButton.onClick = () => {
      Telegram.shareUrl(`Попробуй эту идею: ${currentIdea}`);
    };
    
  } catch (error) {
    console.error('Error:', error);
    ideaText.textContent = 'Ошибка 😢 Попробуй ещё раз';
    ideaText.classList.remove('loading');
  } finally {
    generateBtn.disabled = false;
    generateBtn.textContent = 'Сгенерировать идею ✨';
  }
}

async function sendFeedback(type) {
  if (!currentIdea) return;
  
  try {
    await fetch(`${API_URL}/api/feedback`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ idea: currentIdea, feedback: type })
    });
    
    // Show feedback
    const btn = type === 'like' ? likeBtn : dislikeBtn;
    btn.textContent = type === 'like' ? '✅' : '❌';
    setTimeout(() => {
      btn.textContent = type === 'like' ? '👍' : '👎';
    }, 1000);
    
  } catch (error) {
    console.error('Feedback error:', error);
  }
}

async function loadStats() {
  try {
    const response = await fetch(`${API_URL}/api/stats`);
    const data = await response.json();
    stats.count = data.count || 0;
    statsCount.textContent = stats.count;
  } catch (error) {
    console.error('Stats error:', error);
  }
}
