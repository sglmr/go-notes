-- Insert 15 test notes with hashtags in the text and matching tags in the array

INSERT INTO notes (
        favorite,
        created_at,
        modified_at,
        archive,
        tags,
        title,
        note,
        id
    )
VALUES (
        TRUE,
        NOW(),
        NOW(),
        FALSE,
        ARRAY ['fishing', 'outdoor'],
        'Weekend Plans',
        '# Lake Trip Agenda\n\nGoing to the lake this weekend for some relaxation. Really looking forward to #fishing and enjoying the #outdoor scenery.\n\n## Items to pack:\n* Fishing rods\n* Tackle box\n* Cooler\n* Sunscreen',
        'n_001'
    ),
    (
        FALSE,
        '2025-01-20 14:45:00+00',
        '2025-02-01 09:15:00+00',
        FALSE,
        ARRAY ['cooking', 'recipe'],
        'New Recipe',
        'Found an amazing #recipe for pasta carbonara. My #cooking skills are definitely improving with practice!\n\n**Carbonara Recipe**\n\n> Traditional Italian dish with eggs, cheese, pancetta, and black pepper\n\n```\nIngredients:\n- 8oz spaghetti\n- 2 large eggs\n- 1oz pecorino romano\n- 4oz pancetta\n- Freshly ground black pepper\n```',
        'n_002'
    ),
    (
        TRUE,
        '2025-02-05 11:20:00+00',
        '2025-02-05 16:40:00+00',
        FALSE,
        ARRAY ['work', 'meeting'],
        'Project Deadline',
        '### Q1 Project Timeline\n\nImportant #meeting scheduled for next week. Need to prepare presentation for the #work project before Thursday.\n\n| Task | Deadline | Status |\n|------|----------|--------|\n| Research | 02/07 | ✅ |\n| Slides | 02/08 | 🔄 |\n| Review | 02/09 | ❌ |\n\n*Remember to include the quarterly metrics!*',
        'n_003'
    ),
    (
        FALSE,
        '2025-02-10 09:00:00+00',
        '2025-02-10 09:00:00+00',
        TRUE,
        ARRAY ['travel', 'vacation'],
        'Summer Vacation Ideas',
        'Researching destinations for summer #vacation. Thinking about Italy or Greece for #travel this year.',
        'n_004'
    ),
    (
        TRUE,
        '2025-02-15 17:30:00+00',
        '2025-02-16 10:25:00+00',
        FALSE,
        ARRAY ['health', 'fitness'],
        'New Workout Routine',
        '# 8-Week #Fitness Plan\n\nStarted a new #fitness program today. Focusing on cardio and strength training for better #health.\n\n1. **Monday**: Upper body + 20 min HIIT\n2. **Tuesday**: Lower body + 30 min run\n3. **Wednesday**: Rest day\n4. **Thursday**: Core + 40 min cycling\n5. **Friday**: Full body circuit\n6. **Sat/Sun**: Active recovery\n\n![Workout Progress Chart](https://example.com/chart.png)\n\n~~ Old routine was too time-consuming ~~',
        'n_005'
    ),
    (
        FALSE,
        '2025-02-20 13:15:00+00',
        '2025-02-20 13:15:00+00',
        FALSE,
        ARRAY ['books', 'reading'],
        'Book Recommendations',
        'Just finished an amazing novel. Looking for new #books to add to my #reading list for the month.',
        'n_006'
    ),
    (
        TRUE,
        '2025-02-25 08:45:00+00',
        '2025-02-28 16:10:00+00',
        FALSE,
        ARRAY ['gardening', 'plants'],
        'Spring Garden Planning',
        'Making plans for spring #gardening. Need to buy seeds and check which #plants will work best in the backyard.',
        'n_007'
    ),
    (
        FALSE,
        '2025-03-01 19:20:00+00',
        '2025-03-01 19:20:00+00',
        FALSE,
        ARRAY ['music', 'concert'],
        'Upcoming Concert',
        'Got tickets to the #concert next month! Can''t wait to enjoy live #music again after so long.',
        'n_008'
    ),
    (
        TRUE,
        NOW(),
        NOW(),
        TRUE,
        ARRAY ['coding', 'database'],
        'PostgreSQL Learning',
        '## #Database Learning Path\n\nWorking on improving my #database skills. The #coding exercises with PostgreSQL are challenging but rewarding.\n\n```sql\n-- Example query I learned today\nSELECT \n  date_trunc(''month'', created_at) AS month,\n  COUNT(*) AS total_notes,\n  SUM(CASE WHEN favorite THEN 1 ELSE 0 END) AS favorite_notes\nFROM notes\nGROUP BY month\nORDER BY month;\n```\n\nNeed to review:\n- [x] Basic queries\n- [x] Joins\n- [ ] Window functions\n- [ ] Performance tuning',
        'n_009'
    ),
    (
        FALSE,
        '2025-03-10 15:45:00+00',
        '2025-03-12 09:30:00+00',
        FALSE,
        ARRAY ['shopping', 'gifts'],
        'Birthday Gift Ideas',
        'Need to go #shopping for mom''s birthday. Looking for #gifts that she would actually use and enjoy.',
        'n_010'
    ),
    (
        TRUE,
        '2025-03-15 12:00:00+00',
        '2025-03-15 12:00:00+00',
        FALSE,
        ARRAY ['pets', 'dogs'],
        'Vet Appointment',
        'Scheduled vet appointment for next week. My #dogs need their annual checkup. #pets require such consistent care!',
        'n_011'
    ),
    (
        FALSE,
        '2025-03-18 08:15:00+00',
        '2025-03-18 08:15:00+00',
        FALSE,
        ARRAY ['photography', 'hiking'],
        'Weekend Hike',
        'Planning a #hiking trip to capture some nature #photography. Hope the weather stays clear on Saturday.',
        'n_012'
    ),
    (
        TRUE,
        '2025-03-20 16:40:00+00',
        '2025-03-20 16:40:00+00',
        FALSE,
        ARRAY ['recipe', 'baking'],
        'Bread Experiment',
        'Trying a new sourdough #recipe tomorrow. My #baking skills have improved a lot since I started practicing weekly.',
        'n_013'
    ),
    (
        FALSE,
        '2025-03-21 11:10:00+00',
        '2025-03-21 11:10:00+00',
        FALSE,
        ARRAY ['movie', 'review'],
        'Film Thoughts',
        'Watched an interesting #movie last night. Should write a detailed #review while it''s still fresh in my mind.',
        'n_014'
    ),
    (
        TRUE,
        '2025-03-21 14:25:00+00',
        '2025-03-21 14:25:00+00',
        FALSE,
        ARRAY ['technology', 'productivity'],
        'New App Discovery',
        'Found a great #productivity app that syncs across devices. New #technology that actually makes life easier!',
        'n_015'
    );