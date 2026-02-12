// Valentine's Day themed - make additional real requests to show matches
const resources = [
    '/protected/api/love-letter.json',
    '/protected/api/heart.json',
    '/protected/css/valentine.css',
    '/protected/js/cupid.js',
    '/protected/img/rose.png'
];

let heartCount = 0;
let xCount = 0;

// Fetch browser URL and display QR code
async function fetchBrowserURL() {
    try {
        const response = await fetch('/browserurl');
        const data = await response.json();
        const browserURL = data.url;
        
        console.log('Browser URL:', browserURL);
        
        // Update link
        document.getElementById('browserLink').href = browserURL;
        
        // Clear the container
        const qrContainer = document.getElementById('qrCode');
        qrContainer.innerHTML = '';
        
        // Wait for QRCode library to be loaded
        if (typeof QRCode === 'undefined') {
            console.error('QRCode library not loaded');
            qrContainer.innerHTML = '<p style="color: #ef4444;">QR Code library failed to load</p>';
            return;
        }
        
        // Generate QR code using QRCode.js library
        try {
            new QRCode(qrContainer, {
                text: browserURL,
                width: 256,
                height: 256,
                colorDark: "#ec4899",  // Pink color matching Valentine's theme
                colorLight: "#ffffff",
                correctLevel: QRCode.CorrectLevel.H
            });
            console.log('QR Code generated successfully');
        } catch (qrError) {
            console.error('QR Code generation error:', qrError);
            qrContainer.innerHTML = '<p style="color: #ef4444;">Failed to generate QR code</p>';
        }
        
    } catch (error) {
        console.error('Failed to fetch browser URL:', error);
        const qrContainer = document.getElementById('qrCode');
        qrContainer.innerHTML = '<p style="color: #ef4444;">Failed to load QR code</p>';
    }
}

// Make actual requests and track their responses
function makeRequest(resource, index) {
    const matchItem = document.querySelectorAll('.match-item')[index];
    const matchResult = document.querySelector(`[data-index="${index}"]`);
    
    fetch(resource)
        .then(response => {
            // If response is OK (200-299), show heart
            if (response.status != 403) {
                matchResult.textContent = '❤️';
                matchItem.classList.remove('pending');
                matchItem.classList.add('matched');
                heartCount++;
                document.getElementById('heartCount').textContent = heartCount;
            } else {
                // If response is error (403, 404, etc), show X
                matchResult.textContent = '❌';
                matchItem.classList.remove('pending');
                matchItem.classList.add('rejected');
                xCount++;
                document.getElementById('xCount').textContent = xCount;
            }
        })
        .catch(() => {
            // Network error or denied - show X
            matchResult.textContent = '❌';
            matchItem.classList.remove('pending');
            matchItem.classList.add('rejected');
            xCount++;
            document.getElementById('xCount').textContent = xCount;
        });
}

// Reset and make all requests
function sendRequests() {
    // Disable button
    const tryAgainBtn = document.getElementById('tryAgainBtn');
    tryAgainBtn.disabled = true;
    tryAgainBtn.textContent = 'Sending...';
    
    // Reset all items to pending
    document.querySelectorAll('.match-item').forEach((item, index) => {
        item.className = 'match-item pending';
        document.querySelector(`[data-index="${index}"]`).textContent = '⏳';
    });
    
    // Reset counters
    heartCount = 0;
    xCount = 0;
    document.getElementById('heartCount').textContent = heartCount;
    document.getElementById('xCount').textContent = xCount;
    
    // Send requests with delay
    resources.forEach((resource, index) => {
        setTimeout(() => {
            makeRequest(resource, index);
            
            // Re-enable button after last request
            if (index === resources.length - 1) {
                setTimeout(() => {
                    tryAgainBtn.disabled = false;
                    tryAgainBtn.textContent = 'Try Again';
                }, 1000);
            }
        }, index * 800);
    });
}

// Make requests after a short delay to show multiple auth requests
window.addEventListener('DOMContentLoaded', () => {
    // Fetch browser URL first
    fetchBrowserURL();
    
    // Set up Try Again button
    document.getElementById('tryAgainBtn').addEventListener('click', sendRequests);
    
    // Initial request send
    setTimeout(() => {
        sendRequests();
    }, 1000);
});
