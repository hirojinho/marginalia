# References — pedagogy evidence base

Single source of truth for the literature that drives marginalia's teaching
decisions. The pedagogy rules (`agent/sandbox.go` `writeAgentsMD`,
`agent/agent.go` `toolsAndRulesPrompt`, `CLAUDE.local.md`), the ADRs, and the
study skills all cite these by **author-year key**; the full citation, a stable
link, and *what each one grounds* live here so a reader never has to chase a
bare "(Sweller 1988)" by hand.

When you add or change a pedagogy rule, cite by the key below and add the entry
here if it's new. Keep it a bibliography — reasoning about *why* a rule exists
belongs in the relevant ADR, not here.

DOIs were verified 2026-05-30. Books/chapters without a DOI are cited in full.

---

## Cited in shipped rules & ADRs

### ausubel-1968
Ausubel, D. P. (1968). *Educational Psychology: A Cognitive View.* Holt,
Rinehart & Winston. (book — no DOI)
→ **grounds:** Rule 2 ("What do you already know about X?" — meaningful learning, prior-knowledge anchoring).

### vygotsky-1978
Vygotsky, L. S. (1978). *Mind in Society: The Development of Higher
Psychological Processes.* Harvard University Press. (book — no DOI)
→ **grounds:** Rule 3 (confidence calibration to the Zone of Proximal Development).

### flavell-1979
Flavell, J. H. (1979). Metacognition and cognitive monitoring: A new area of
cognitive–developmental inquiry. *American Psychologist, 34*(10), 906–911.
https://doi.org/10.1037/0003-066X.34.10.906
→ **grounds:** Rule 3 (metacognitive monitoring — the "how confident are you?" check).

### anderson-1977
Anderson, R. C. (1977). The notion of schemata and the educational enterprise.
In R. C. Anderson, R. J. Spiro, & W. E. Montague (Eds.), *Schooling and the
Acquisition of Knowledge* (pp. 415–431). Erlbaum. (book chapter — no DOI)
→ **grounds:** Rule 4 (connect every new concept to prior knowledge — schema theory).

### anderson-krathwohl-2001
Anderson, L. W., & Krathwohl, D. R. (Eds.). (2001). *A Taxonomy for Learning,
Teaching, and Assessing: A Revision of Bloom's Taxonomy of Educational
Objectives.* Longman. (book — no DOI)
→ **grounds:** Rule 5 (explain → apply → analyze → evaluate → create; Reflect-task Bloom tags `[B-A]/[B-N]/[B-E]/[B-C]`).

### slamecka-graf-1978
Slamecka, N. J., & Graf, P. (1978). The generation effect: Delineation of a
phenomenon. *Journal of Experimental Psychology: Human Learning and Memory,
4*(6), 592–604. https://doi.org/10.1037/0278-7393.4.6.592
→ **grounds:** Rule 7 (pre-read prediction); Reflect-task generation rule (withhold the answer — producing > recognizing).

### chi-1989
Chi, M. T. H., Bassok, M., Lewis, M. W., Reimann, P., & Glaser, R. (1989).
Self-explanations: How students study and use examples in learning to solve
problems. *Cognitive Science, 13*(2), 145–182.
https://doi.org/10.1207/s15516709cog1302_1
→ **grounds:** [ADR 0012](adr/0012-segmented-active-reading.md) and Rule 9 (self-explanation at chunk boundaries).

### sweller-1988
Sweller, J. (1988). Cognitive load during problem solving: Effects on learning.
*Cognitive Science, 12*(2), 257–285.
https://doi.org/10.1207/s15516709cog1202_4
→ **grounds:** Rule 1 (no continuous lecturing), Rule 8 (3-new-terms budget), and [ADR 0012](adr/0012-segmented-active-reading.md) (segmenting dense readings). Cognitive Load Theory.

### roediger-karpicke-2006
Roediger, H. L., & Karpicke, J. D. (2006). Test-enhanced learning: Taking memory
tests improves long-term retention. *Psychological Science, 17*(3), 249–255.
https://doi.org/10.1111/j.1467-9280.2006.01693.x
→ **grounds:** Rule 6 (session-open retrieval check). The testing effect — highest-evidence move in the Dunlosky 2013 review.

### rohrer-taylor-2007
Rohrer, D., & Taylor, K. (2007). The shuffling of mathematics problems improves
learning. *Instructional Science, 35*(6), 481–498.
https://doi.org/10.1007/s11251-007-9015-8
→ **grounds:** Rule 6 (occasional interleaved retrieval of an older task). Interleaving.

### cepeda-2008
Cepeda, N. J., Vul, E., Rohrer, D., Wixted, J. T., & Pashler, H. (2008). Spacing
effects in learning: A temporal ridgeline of optimal retention. *Psychological
Science, 19*(11), 1095–1102. https://doi.org/10.1111/j.1467-9280.2008.02209.x
→ **grounds:** Rule 6 (spaced retrieval); the "small, spaced unit" Session shape ([ADR 0009](adr/0009-session-single-task-spaced-unit.md)).

### richland-kornell-kao-2009
Richland, L. E., Kornell, N., & Kao, L. S. (2009). The pretesting effect: Do
unsuccessful retrieval attempts enhance learning? *Journal of Experimental
Psychology: Applied, 15*(3), 243–257. https://doi.org/10.1037/a0016496
→ **grounds:** Rule 7 (predict → **STOP**, no same-turn reveal) and Rule 9 / [ADR 0015](adr/0015-silent-grounding-tutor-withholds-resource.md) (withhold content until the learner has read it). The pretesting effect — pre-exposing the answer destroys it.

### kornell-hays-bjork-2009
Kornell, N., Hays, M. J., & Bjork, R. A. (2009). Unsuccessful retrieval attempts
enhance subsequent learning. *Journal of Experimental Psychology: Learning,
Memory, and Cognition, 35*(4), 989–998. https://doi.org/10.1037/a0015729
→ **grounds:** Rule 9 / [ADR 0015](adr/0015-silent-grounding-tutor-withholds-resource.md) (a failed prediction *before* reading still enhances learning — so withholding is productive, not just polite).

### bjork-bjork-2011
Bjork, E. L., & Bjork, R. A. (2011). Making things hard on yourself, but in a
good way: Creating desirable difficulties to enhance learning. In M. A.
Gernsbacher et al. (Eds.), *Psychology and the Real World* (pp. 56–64). Worth.
https://bjorklab.psych.ucla.edu/wp-content/uploads/sites/13/2016/07/EBjork_RBjork_2011.pdf
→ **grounds:** Rule 9 / [ADR 0015](adr/0015-silent-grounding-tutor-withholds-resource.md) (handing the answer over removes the desirable difficulty that does the work).

### dunlosky-2013
Dunlosky, J., Rawson, K. A., Marsh, E. J., Nathan, M. J., & Willingham, D. T.
(2013). Improving students' learning with effective learning techniques:
Promising directions from cognitive and educational psychology. *Psychological
Science in the Public Interest, 14*(1), 4–58.
https://doi.org/10.1177/1529100612453266
→ **grounds:** Rule 6 (testing effect ranked top) and Rule 9 / [ADR 0012](adr/0012-segmented-active-reading.md) (passive rereading/highlighting is low-utility — don't tell a stuck learner to "reread").

### berlyne-1960
Berlyne, D. E. (1960). *Conflict, Arousal, and Curiosity.* McGraw-Hill. (book — no DOI)
→ **grounds:** Interest-log surfacing rule (curiosity as the engine of self-directed learning; close-or-pursue so the log isn't psychic debt).

---

## Queued — ground R-backlog items not yet shipped

Cited in `ROADMAP.md`'s pedagogy backlog (R1–R8); listed here so the bibliography
stays complete. Links will be filled in as each item ships.

### bloom-1968
Bloom, B. S. (1968). Learning for mastery. *Evaluation Comment, 1*(2).
→ **grounds:** R3 (mastery gates — a cluster is done only at sustained retrieval confidence).

### sweller-cooper-1985
Sweller, J., & Cooper, G. A. (1985). The use of worked examples as a substitute
for problem solving in learning algebra. *Cognition and Instruction, 2*(1),
59–89. https://doi.org/10.1207/s1532690xci0201_3
→ **grounds:** R5 (worked-example → completion pairs for computational topics).

### renkl-2014
Renkl, A. (2014). The worked-examples principle in multimedia learning. In R. E.
Mayer (Ed.), *The Cambridge Handbook of Multimedia Learning* (2nd ed.).
Cambridge University Press.
→ **grounds:** R5 (worked-example principle).

### carvalho-goldstone-2014
Carvalho, P. F., & Goldstone, R. L. (2014). Putting category learning in order:
Category structure and temporal arrangement affect the benefit of interleaved
over blocked study. *Memory & Cognition, 42*(3), 481–495.
→ **grounds:** R6 (interleaving by default).

### ericsson-1993
Ericsson, K. A., Krampe, R. T., & Tesch-Römer, C. (1993). The role of deliberate
practice in the acquisition of expert performance. *Psychological Review,
100*(3), 363–406. https://doi.org/10.1037/0033-295X.100.3.363
→ **grounds:** R-Later (deliberate-practice tracks).
