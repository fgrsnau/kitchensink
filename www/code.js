async function call_increment(user_id) {
	try {
		await $.post('/api/increment?user=' + user_id);
	} catch (err) {
		if (err.status == 429) {
			alert(`No cheating is allowed!\n\n(Got response: ${err.status} ${err.statusText})`);
		} else {
			alert(`Error during RPC call!\n\n${err}`);
		}
	}

	update_ui_total();
	update_ui_last();
}

async function update_ui_total() {
	const helper = function(element, counter) {
		if (counter == 1) {
			element.text(`${counter} time`);
		} else {
			element.text(`${counter} times`);
		}
	};

	const response = await $.get('/api/total');
	helper($('#today'), response.today);
	helper($('#this_week'), response.this_week);
}

async function update_ui_last() {
	const response = await $.get('/api/last');

	const last = $('#last');
	if (response.elapsed > 24*60) {
		last.text('more than a day before');
	} else {
		const minutes = response.elapsed | 0;
		if (minutes == 1) {
			last.text(`${minutes} minute ago`);
		} else {
			last.text(`${minutes} minutes ago`);
		}
	}

	last.css('color', '');
	if (response.elapsed < 45) {
		last.css('color', 'green');
	}
	if (response.elapsed > 2 * 60) {
		last.css('color', 'red');
	}
}

async function update_ui_persons() {
	const users = await $.get('/api/users');
	const persons = $('#persons');	
	persons.html('');
	var total_cleaned = 0;
	for (const i in users) {
		const button = $('<button>');
		button.text(users[i].name);
		button.click(function () { call_increment(users[i].id); });
		persons.append(button);
	}
}

function update_ui() {
	update_ui_last();
	update_ui_persons();
	update_ui_total();
}

$(document).ready(function() {
	$('body').click(function () { document.documentElement.requestFullscreen(); });
	setTimeout(update_ui, 0);
	setInterval(update_ui, 10000);
});
