// Ezhik Ideas — Frontend App (Robust Version)

const API_URL = window.location.origin;

// Telegram WebApp integration
const Telegram = window.Telegram.WebApp;
if (Telegram) {
  Telegram.expand();
  Telegram.ready();
}

let currentIdea = '';
let stats = { count: 0 };
let history = [];

// DOM elements (will be assigned in init)
let generateBtn, categorySelect, ideaBox, ideaText, actions, likeBtn, dislikeBtn, regenBtn, statsCount, historySection, historyList;

document.addEventListener('DOMContentLoaded', init);

function init() {
  // Assign elements
  generateBtn = document.getElementById('generate-btn');
  categorySelect = document.getElementById('category');
  ideaBox = document.getElementById('idea-box');
  ideaText = document.getElementById('idea-text');
  actions = document.getElementById('actions');
  likeBtn = document.getElementById('like-btn');
  dislikeBtn = document.getElementById('dislike-btn');
  regenBtn = document.getElementById('regen-btn');
  statsCount = document.getElementById('stats-count');
  historySection = document.getElementById('history-section');
  historyList = document.getElementById('history-list');

  // Load from storage
  try {
    const saved = localStorage.getItem('ezhik_history_v2');
    if (saved) {
      history = JSON.parse(saved);
    }
  } catch (e) {
    console.error('Storage load error:', e);
    history = [];
  }

  // Event listeners
  if (generateBtn) generateBtn.addEventListener('click', generateIdea);
  if (regenBtn) regenBtn.addEventListener('click', generateIdea);
  if (likeBtn) likeBtn.addEventListener('click', () => sendFeedback('like'));
  if (dislikeBtn) dislikeBtn.addEventListener('click', () => sendFeedback('dislike'));

  loadStats();
  renderHistory();
}

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
    const response = await fetch(`${API_URL}/api/idea?category=${encodeURIComponent(category)}`);
    if (!response.ok) throw new Error('API status: ' + response.status);
    
    const data = await response.json();
    currentIdea = data.idea;
    
    // Update UI
    ideaText.textContent = currentIdea;
    ideaText.classList.remove('loading');
    actions.classList.remove('hidden');
    
    // Update local stats
    stats.count++;
    if (statsCount) statsCount.textContent = stats.count;
    
    // Update history
    saveToHistory(currentIdea, category);

    if (Telegram && Telegram.MainButton) {
      Telegram.MainButton.text = 'Поделиться';
      Telegram.MainButton.show();
      Telegram.MainButton.onClick(() => {
        Telegram.shareUrl(`Попробуй эту идею: ${currentIdea}`);
      });
    }
    
  } catch (error) {
    console.error('Error:', error);
    ideaText.textContent = 'Ошибка 😢 Попробуй ещё раз. ' + error.message;
    ideaText.classList.remove('loading');
  } finally {
    generateBtn.disabled = false;
    generateBtn.textContent = 'Сгенерировать идею ✨';
  }
}

function saveToHistory(idea, category) {
  const item = { 
    idea, 
    category, 
    date: new Date().toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) 
  };
  
  history.unshift(item);
  if (history.length > 10) history.pop();
  
  try {
    localStorage.setItem('ezhik_history_v2', JSON.stringify(history));
  } catch (e) {
    console.error('Storage save error:', e);
  }
  
  renderHistory();
}

function renderHistory() {
  if (!historyList || !historySection) return;
  
  if (history.length === 0) {
    historySection.classList.add('hidden');
    return;
  }
  
  historySection.classList.remove('hidden');
  
  let html = '';
  for (const item of history) {
    html += `
      <li style="margin-bottom: 10px; padding-bottom: 5px; border-bottom: 1px solid rgba(255,255,255,0.1)">
        <div style="font-size: 10px; color: #6366f1; font-weight: bold; text-transform: uppercase;">
          ${item.category} • ${item.date}
        </div>
        <div style="font-size: 14px; color: #e2e8f0; margin-top: 3px;">
          ${item.idea}
        </div>
      </li>`;
  }
  historyList.innerHTML = html;
}

async function sendFeedback(type) {
  if (!currentIdea) return;
  const btn = type === 'like' ? likeBtn : dislikeBtn;
  
  try {
    await fetch(`${API_URL}/api/feedback`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ idea: currentIdea, feedback: type })
    });
    
    const oldText = btn.textContent;
    btn.textContent = '✅';
    setTimeout(() => { btn.textContent = oldText; }, 1000);
  } catch (error) {
    console.error('Feedback error:', error);
  }
}

async function loadStats() {
  try {
    const response = await fetch(`${API_URL}/api/stats`);
    const data = await response.json();
    stats.count = data.count || 0;
    if (statsCount) statsCount.textContent = stats.count;
  } catch (error) {
    console.error('Stats error:', error);
  }
}
