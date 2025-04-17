CREATE TABLE sentence (                                                  
   id INTEGER PRIMARY KEY,                                           
   text TEXT NOT NULL
);                                                                    

CREATE TABLE person_who_types (                                                  
   ssh_finger_print TEXT PRIMARY KEY,                                               
   typing_test_completion_count INTEGER NOT NULL
);                                                                    


CREATE INDEX idx_typing_test_completion_count 
ON person_who_types (typing_test_completion_count);                                       
